// The core game client. It owns the WebSocket: HTML frames from the server
// are handed to htmx, which applies their out-of-band swaps into the page's
// slots (#feed for now), reply frames resolve requests, and event frames
// go to plugins' listeners. See docs/design.md §5.
//
// Plugins' JavaScript uses it through the import map:
//
//   import { client } from "dragon";
//
//   client.send("cast fireball goblin");             // a command
//   client.push("mapping:pan", { x: 4 });             // a plugin's client event
//   const area = await client.request("mapping:area", { id: "riverside" });
//   client.on("mapping:path_found", (data) => ...);   // pushed by session:push
//   client.onMessage("room", (element) => ...);       // a message in the feed
//   client.connection.on("connected", () => ...);     // also "disconnected"
//   client.hook("mapping-pin", { mounted(el) {}, removed(el) {} });

const MAX_FEED_LINES = 2000;
const RECONNECT_DELAY_MS = 2000;
const REQUEST_TIMEOUT_MS = 5000;

const feed = document.getElementById("feed");
const form = document.getElementById("command");
const input = document.getElementById("input");
const status = document.getElementById("status");

let socket = null;

// listeners maps a name to the functions listening for it: event names
// for client.on, "message:<kind>" for client.onMessage and
// "connection:<state>" for client.connection.on.
const listeners = new Map();

function listen(name, handler) {
  if (!listeners.has(name)) listeners.set(name, new Set());
  listeners.get(name).add(handler);
  return () => listeners.get(name)?.delete(handler);
}

function emit(name, ...args) {
  for (const handler of listeners.get(name) ?? []) {
    try {
      handler(...args);
    } catch (err) {
      console.error(`dragon: a ${name} listener failed`, err);
    }
  }
}

function connect() {
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  socket = new WebSocket(`${scheme}://${location.host}/ws`);

  socket.addEventListener("open", () => {
    setStatus("Connected", "connected");
    emit("connection:connected");
  });

  socket.addEventListener("message", (event) => {
    const frame = JSON.parse(event.data);
    if (frame.t === "html") {
      const stick = nearBottom();
      const last = feed.lastElementChild;
      htmx.swap(feed, frame.html, { swapStyle: "none" });
      for (let el = last ? last.nextElementSibling : feed.firstElementChild; el; el = el.nextElementSibling) {
        if (el.dataset.kind) emit(`message:${el.dataset.kind}`, el);
      }
      trimFeed();
      if (stick) feed.scrollTop = feed.scrollHeight;
      if (frame.secret) setSecret(true);
    } else if (frame.t === "reply") {
      pending.get(frame.id)?.resolve("data" in frame ? frame.data : frame.html ?? "");
      pending.delete(frame.id);
    } else if (frame.t === "event") {
      emit(frame.name, frame.data ?? null);
    }
  });

  socket.addEventListener("close", () => {
    for (const { reject } of pending.values()) reject(new Error("disconnected"));
    pending.clear();
    setStatus("Disconnected. Reconnecting…", "disconnected");
    emit("connection:disconnected");
    setTimeout(connect, RECONNECT_DELAY_MS);
  });
}

function send(line) {
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ t: "cmd", line }));
  }
}

// Requests ask the server for something and resolve with its reply: a
// plugin's client event handler's answer, or HTML for the engine's own
// requests, such as tooltips.
const pending = new Map();
let lastRequestID = 0;

// push sends a plugin's client event without waiting for an answer.
function push(name, data = {}) {
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ t: "req", name, data }));
  }
}

function request(name, data = {}) {
  if (socket?.readyState !== WebSocket.OPEN) {
    return Promise.reject(new Error("not connected"));
  }

  const id = String(++lastRequestID);
  socket.send(JSON.stringify({ t: "req", id, name, data }));

  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    setTimeout(() => {
      if (pending.delete(id)) reject(new Error(`${name} timed out`));
    }, REQUEST_TIMEOUT_MS);
  });
}

function setStatus(text, state) {
  status.textContent = text;
  status.dataset.state = state;
}

function nearBottom() {
  return feed.scrollHeight - feed.scrollTop - feed.clientHeight < 40;
}

function trimFeed() {
  while (feed.childElementCount > MAX_FEED_LINES) {
    feed.firstElementChild.remove();
  }
}

// Echo what the player typed, like a telnet client does.
function echo(line) {
  const div = document.createElement("div");
  div.className = "msg msg-echo";
  div.textContent = line;
  feed.append(div);
  feed.scrollTop = feed.scrollHeight;
}

// Secret input, such as a password: hidden as it's typed, not echoed and
// not kept in history. It lasts for one line.
let secret = false;

function setSecret(on) {
  secret = on;
  input.type = on ? "password" : "text";
  input.placeholder = on ? "" : "Type a command and press Enter";
}

// Command history with the up and down arrows.
const history = [];
let historyIndex = 0;

form.addEventListener("submit", (event) => {
  event.preventDefault();
  const line = input.value;
  input.value = "";

  if (secret) {
    setSecret(false);
    send(line);
    return;
  }

  echo(line);
  send(line);

  if (line.trim() !== "" && history.at(-1) !== line) history.push(line);
  historyIndex = history.length;
});

input.addEventListener("keydown", (event) => {
  if (secret) return;
  if (event.key === "ArrowUp" && historyIndex > 0) {
    historyIndex--;
  } else if (event.key === "ArrowDown" && historyIndex < history.length) {
    historyIndex++;
  } else {
    return;
  }

  event.preventDefault();
  input.value = history[historyIndex] ?? "";
});

// Clicking the feed shouldn't lose the input's focus, unless selecting text.
feed.addEventListener("mouseup", () => {
  if (!window.getSelection()?.toString()) input.focus();
});

// Entities: <dragon-entity ref="id">name</dragon-entity> in server HTML.
// Hovering, focusing or long-pressing one shows its tooltip; clicking it,
// or Enter or Space, runs its default action. The server decides both,
// through the get_tooltip and get_default_action hooks.

const TOOLTIP_DELAY_MS = 350;
const LONG_PRESS_MS = 500;

const tooltip = document.createElement("div");
tooltip.id = "dragon-tooltip";
tooltip.className = "dragon-tooltip";
tooltip.setAttribute("role", "tooltip");
tooltip.hidden = true;
document.body.append(tooltip);

let tooltipFor = null;
let tooltipTimer = null;

function showTooltip(entity) {
  clearTimeout(tooltipTimer);
  tooltipTimer = setTimeout(async () => {
    tooltipFor = entity;
    let html = "";
    try {
      html = await request("entity_tooltip", { ref: entity.ref });
    } catch {
      return;
    }
    // The pointer or focus may have moved on while waiting.
    if (tooltipFor !== entity || html.trim() === "") return;

    tooltip.innerHTML = html;
    tooltip.hidden = false;
    entity.setAttribute("aria-describedby", tooltip.id);
    placeTooltip(entity);
  }, TOOLTIP_DELAY_MS);
}

function hideTooltip(entity) {
  clearTimeout(tooltipTimer);
  if (entity && tooltipFor !== entity) return;
  tooltipFor?.removeAttribute("aria-describedby");
  tooltipFor = null;
  tooltip.hidden = true;
}

// Below the entity, or above it when there's no room, kept on screen.
function placeTooltip(entity) {
  const gap = 6;
  const r = entity.getBoundingClientRect();
  const t = tooltip.getBoundingClientRect();
  let top = r.bottom + gap;
  if (top + t.height > window.innerHeight && r.top - gap - t.height > 0) {
    top = r.top - gap - t.height;
  }
  const left = Math.max(gap, Math.min(r.left, window.innerWidth - t.width - gap));
  tooltip.style.top = `${top}px`;
  tooltip.style.left = `${left}px`;
}

function act(entity) {
  hideTooltip();
  request("entity_action", { ref: entity.ref }).catch(() => {});
}

class DragonEntity extends HTMLElement {
  get ref() {
    return this.getAttribute("ref") ?? "";
  }

  connectedCallback() {
    if (this.hasAttribute("tabindex")) return; // already set up
    this.setAttribute("role", "button");
    this.tabIndex = 0;

    let pressTimer = null;
    let longPressed = false;

    this.addEventListener("mouseenter", () => showTooltip(this));
    this.addEventListener("mouseleave", () => hideTooltip(this));
    this.addEventListener("focus", () => showTooltip(this));
    this.addEventListener("blur", () => hideTooltip(this));

    // Touch screens have no hover: a long press shows the tooltip instead
    // of acting.
    this.addEventListener("pointerdown", (event) => {
      if (event.pointerType !== "touch") return;
      longPressed = false;
      pressTimer = setTimeout(() => {
        longPressed = true;
        showTooltip(this);
      }, LONG_PRESS_MS);
    });
    for (const type of ["pointerup", "pointercancel", "pointerleave"]) {
      this.addEventListener(type, () => clearTimeout(pressTimer));
    }
    this.addEventListener("contextmenu", (event) => {
      if (longPressed) event.preventDefault();
    });

    this.addEventListener("click", (event) => {
      event.stopPropagation();
      if (longPressed) {
        longPressed = false;
        return;
      }
      act(this);
    });
    this.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        act(this);
      } else if (event.key === "Escape") {
        hideTooltip();
      }
    });
  }
}

customElements.define("dragon-entity", DragonEntity);

// Commands: <dragon-command value="go north">north</dragon-command>, which
// {{command "go north" "north"}} writes. Clicking one, or Enter or Space,
// runs its value exactly as if the player had typed it.
//
// Choices: <dragon-choice value="warrior">Warrior</dragon-choice> in a
// prompt work the same way, answering with their value.
class DragonCommand extends HTMLElement {
  connectedCallback() {
    if (this.hasAttribute("tabindex")) return; // already set up
    this.setAttribute("role", "button");
    this.tabIndex = 0;

    const choose = () => {
      const value = this.getAttribute("value") ?? this.textContent;
      echo(value);
      send(value);
      input.focus();
    };
    this.addEventListener("click", (event) => {
      event.stopPropagation();
      choose();
    });
    this.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        choose();
      }
    });
  }
}

customElements.define("dragon-command", DragonCommand);
customElements.define("dragon-choice", class DragonChoice extends DragonCommand {});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") hideTooltip();
});
feed.addEventListener("scroll", () => hideTooltip());

// Hooks give plain elements a lifecycle, as connectedCallback does for
// custom elements: an element with data-dragon-hook="name" calls the
// hook's mounted(element) when it's added to the page and removed(element)
// when it leaves.
const hooks = new Map();

function hook(name, callbacks) {
  hooks.set(name, callbacks);
  for (const el of document.querySelectorAll(`[data-dragon-hook="${CSS.escape(name)}"]`)) {
    callbacks.mounted?.(el);
  }
}

function runHooks(node, which) {
  if (!(node instanceof Element)) return;
  const els = node.matches("[data-dragon-hook]") ? [node] : [];
  els.push(...node.querySelectorAll("[data-dragon-hook]"));
  for (const el of els) {
    try {
      hooks.get(el.dataset.dragonHook)?.[which]?.(el);
    } catch (err) {
      console.error(`dragon: hook ${el.dataset.dragonHook} failed`, err);
    }
  }
}

new MutationObserver((mutations) => {
  for (const m of mutations) {
    m.addedNodes.forEach((node) => runHooks(node, "mounted"));
    m.removedNodes.forEach((node) => runHooks(node, "removed"));
  }
}).observe(document.body, { childList: true, subtree: true });

export const client = {
  send,
  push,
  request,
  on: listen,
  onMessage: (kind, handler) => listen(`message:${kind}`, handler),
  connection: { on: (state, handler) => listen(`connection:${state}`, handler) },
  hook,
};
window.dragon = { client };

connect();
