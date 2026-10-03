package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"bbuck.dev/dragon-mud/ansi"
	"bbuck.dev/dragon-mud/random"
)

// idleAfter is how long the logs can be quiet before the dragon says
// something, so you know the server is still alive.
const idleAfter = 15 * time.Minute

// rareOdds is the chance, 1 in rareOdds, that a legendary dragon answers
// the summons instead of an ordinary one.
const rareOdds = 100

// dragon is the dragon that minds the server for one run of dragon serve.
// It greets you, comments on reloads and says goodbye. This is flavor, not
// output, so it writes to stderr with the logs, and dragon = false in
// dragon.toml sends it away.
type dragon struct {
	// title introduces it and name refers to it afterwards, both with
	// color codes: "A [R]red[x] dragon" and "The [R]red[x] dragon".
	title, name string

	// mu guards rng and out: the dragon speaks from the main goroutine,
	// the script watcher and its own idle timer.
	mu    sync.Mutex
	rng   *random.Rand
	out   io.Writer
	color bool
}

var dragons = []string{
	"[l][-W]black[x]", "[c220]brass[x]", "[R]red[x]", "[c208]bronze[x]", "[G]green[x]",
	"[Y]gold[x]", "[B]blue[x]", "[c202]copper[x]", "[W]white[x]", "[c250][u]silver[x]",
}

// legends are the rare dragons, as { title, name }.
var legends = [][2]string{
	{"The five-headed queen of [R]red[x], [B]blue[x], [G]green[x], [l][-W]black[x] and [W]white[x] dragons",
		"The dragon queen"},
	{"The [W]platinum[x] dragon king, eldest of the metallic dragons,", "The dragon king"},
	{"An [c094]ancient[x] dragon, older than the mountains,", "The ancient dragon"},
}

// arrivals finish "A red dragon ...", before its duty.
var arrivals = []string{
	"arrives",
	"uncoils from its hoard",
	"descends from the clouds",
	"wakes beneath the mountain",
	"lands with a thunderous thump",
	"slips out of the shadows",
	"stretches its wings",
	"emerges from a curl of smoke",
}

// duties finish the greeting; %s is the game's name.
var duties = []string{
	"to serve you today",
	"to guard %s",
	"to watch over %s",
	"to keep the fires of %s lit",
	"to stand watch over %s",
	"to tend %s",
	"to count the coins of %s",
	"to settle in for a long watch over %s",
}

// reloads follow the dragon's name when scripts change.
var reloads = []string{
	"sniffs at your changes and nods.",
	"rereads its scrolls.",
	"rearranges its hoard to match.",
	"blinks, and the world shifts a little.",
	"taps the new spells with a claw. They hold.",
	"mutters the changes under its breath.",
}

// farewells follow the dragon's name when the server stops.
var farewells = []string{
	"curls up atop its hoard and sleeps.",
	"flies off into the night.",
	"banks the fires and settles in to wait.",
	"returns to its lair.",
	"folds its wings and turns to stone until it's needed.",
	"yawns, a little smoke escaping, and drifts off.",
}

// idles follow the dragon's name when the logs have been quiet for a
// while.
var idles = []string{
	"stirs in its sleep. All is quiet.",
	"opens one eye, sees nothing amiss, and closes it again.",
	"counts its hoard. Nothing is missing.",
	"listens to the quiet halls.",
	"shifts on its coins with a soft clink.",
	"breathes a lazy curl of smoke.",
}

// summon picks the dragon for this run.
func summon(rng *random.Rand, out io.Writer, color bool) *dragon {
	d := &dragon{rng: rng, out: out, color: color}

	if rng.Range(1, rareOdds) == 1 {
		legend := legends[rng.Range(0, len(legends)-1)]
		d.title, d.name = legend[0], legend[1]
		return d
	}

	kind := pick(rng, dragons)
	d.title, d.name = "A "+kind+" dragon", "The "+kind+" dragon"

	return d
}

// gameMark stands in for the game's name until color codes are applied,
// so codes in the name are shown as written.
const gameMark = "\x00"

// greet announces the dragon, the game and how many things are in its
// world.
func (d *dragon) greet(game string, objects int) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	duty := pick(d.rng, duties)
	if strings.Contains(duty, "%s") {
		duty = fmt.Sprintf(duty, gameMark)
	}

	var world string
	switch objects {
	case 0:
		world = "The world is empty, waiting to be built."
	case 1:
		world = "The one thing in the world is where it left it."
	default:
		world = fmt.Sprintf("All %d things in the world are where it left them.", objects)
	}

	line := d.render(d.title + " " + pick(d.rng, arrivals) + " " + duty + ". " + world)
	fmt.Fprintln(d.out, strings.ReplaceAll(line, gameMark, game))
}

// reloaded remarks on scripts reloading.
func (d *dragon) reloaded() {
	d.say(reloads, "")
}

// farewell says goodbye as the server stops.
func (d *dragon) farewell() {
	d.say(farewells, "")
}

// keepWatch has the dragon speak whenever nothing has been logged for
// idleAfter, saying how long it's been watching, until ctx is done. Its
// own words count as activity, so it speaks at most once per idleAfter.
func (d *dragon) keepWatch(ctx context.Context, logs activity, idleAfter time.Duration) error {
	if d == nil {
		return nil
	}

	started := time.Now()
	for {
		wait := idleAfter - logs.quiet()
		if wait <= 0 {
			d.say(idles, " It has kept watch for "+since(started)+".")
			logs.touch()
			wait = idleAfter
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// say has the dragon say one of lines, then after.
func (d *dragon) say(lines []string, after string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	fmt.Fprintln(d.out, d.render(d.name+" "+pick(d.rng, lines)+after))
}

// since describes how long it's been since start, to the minute.
func since(start time.Time) string {
	d := time.Since(start).Round(time.Minute)
	days, hours, minutes := int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60

	unit := func(n int, name string) string {
		if n == 1 {
			return "1 " + name
		}
		return fmt.Sprintf("%d %ss", n, name)
	}

	switch {
	case days > 0 && hours > 0:
		return unit(days, "day") + " and " + unit(hours, "hour")
	case days > 0:
		return unit(days, "day")
	case hours > 0 && minutes > 0:
		return unit(hours, "hour") + " and " + unit(minutes, "minute")
	case hours > 0:
		return unit(hours, "hour")
	case minutes == 0:
		return "less than a minute"
	default:
		return unit(minutes, "minute")
	}
}

// render applies line's color codes, or removes them without color.
func (d *dragon) render(line string) string {
	if d.color {
		return ansi.Colorize(line + "[x]")
	}

	return ansi.Purge(line)
}

func pick(rng *random.Rand, options []string) string {
	return options[rng.Range(0, len(options)-1)]
}
