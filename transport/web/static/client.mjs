// The core game client. It owns the WebSocket: HTML frames from the server
// are handed to htmx, which applies their out-of-band swaps into the page's
// slots (#feed for now). See docs/design.md §5.

const MAX_FEED_LINES = 2000;
const RECONNECT_DELAY_MS = 2000;

const feed = document.getElementById("feed");
const form = document.getElementById("command");
const input = document.getElementById("input");
const status = document.getElementById("status");

let socket = null;

function connect() {
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  socket = new WebSocket(`${scheme}://${location.host}/ws`);

  socket.addEventListener("open", () => setStatus("Connected", "connected"));

  socket.addEventListener("message", (event) => {
    const frame = JSON.parse(event.data);
    if (frame.t === "html") {
      const stick = nearBottom();
      htmx.swap(feed, frame.html, { swapStyle: "none" });
      trimFeed();
      if (stick) feed.scrollTop = feed.scrollHeight;
    }
  });

  socket.addEventListener("close", () => {
    setStatus("Disconnected. Reconnecting…", "disconnected");
    setTimeout(connect, RECONNECT_DELAY_MS);
  });
}

function send(line) {
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ t: "cmd", line }));
  }
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

// Command history with the up and down arrows.
const history = [];
let historyIndex = 0;

form.addEventListener("submit", (event) => {
  event.preventDefault();
  const line = input.value;
  input.value = "";

  echo(line);
  send(line);

  if (line.trim() !== "" && history.at(-1) !== line) history.push(line);
  historyIndex = history.length;
});

input.addEventListener("keydown", (event) => {
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

export const client = { send };
window.dragon = { client };

connect();
