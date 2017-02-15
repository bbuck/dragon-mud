# DragonMUD

[![Build Status](https://travis-ci.org/bbuck/dragon-mud.svg?branch=develop)](https://travis-ci.org/bbuck/dragon-mud)
[![Code Climate](https://codeclimate.com/github/bbuck/dragon-mud/badges/gpa.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Issue Count](https://codeclimate.com/github/bbuck/dragon-mud/badges/issue_count.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Discord Channel](https://img.shields.io/badge/discord-DragonMUD-blue.svg?style=flat)](https://discordapp.com/channels/141274099262423040/141274099262423040)
[![Go Report Card](https://goreportcard.com/badge/github.com/bbuck/dragon-mud)](https://goreportcard.com/report/github.com/bbuck/dragon-mud)

DragonMUD is a dream of mine, building a new MUD engine for experience and fun
before building my own game on top of it. The engine will have a very opinionated
idea of game rules (such as how skills works, etc.) but outside of game rules
will be entirely customizable through scripts (using Lua) and being open source
would be open to forks with custom game rules implemented.

### Why should I use this?

That's really up to you. This project is for me but I believe in sharing. I
also feel a new "modern" MUD engine may inspire some new games to be created
in the genre which would be amazing. I'm a huge fan of MUDs and feel that new
entries have kind of become almost non-existent. Perhaps a new game and an
accessible low-setup MUD server would make it easier for new games to be quickly
created.

### What exactly is it?

DragonMUD is a MUD server for text based multi-user games. It will be pretty
biased on how skills and in game systems works as the core of DragonMUD is to
power a MUD game that I, myself, have planned. That leads the implementation to
be driven towards that end. Now, I'm not completely selfish - at any point if I
can make the the game server configurable I will gladly do that.

### What exactly is it not?

DragonMUD is not a "game engine" for games in general. It's heavily geared to
building an "MMO with no graphics" that runs over Telnet and is delivered in
colored text. If you're looking for a game engine in Go to build a 2D platformer
or a Call of Duty shooter you're in the wrong place.

## Why Go?

I love C and C++ but they're older and slightly more complex languages to set
up and maintain. [Go](https://golang.org/) strives to be a "modern" C and was
therefore a good choice in my opinion. It also supports concurrency out of the
box in a very easy to use and understand way. It's also relatively low level
enough for the purpose of a game server running.

## Why Lua?

I wanted to write my own language, and for a Ruby attempt at this project I
actually did. You can find it [here](https://github.com/bbuck/eleetscript) but
it was a lot of work, has a lot of holes and is relatively unfamiliar to anyone
who may or may not be used to scripting games. On the other hand, Lua has been
around. It's been tested and it has a slew of core features that would be
great to leverage. It's also very common among games as a scripting layer and
became a prime choice to replace a custom built engine.

# Roadmap

I have grandiose plans. At the moment, they're not divided into versions but as
this project matures I will clean up and define these details more and more.

 - [x] TravisCI integration to easily demonstrate stable builds
 - [x] Code climate monitoring GPA of code, maintaining an A - B grade for overall
   project
 - [x] Test suites with extensive coverage (protected with Coveralls) -- (not
   'completed' per se, but started)
 - [ ] Database backed server, can choose which database (Neo4j)
   - [x] Partial implementation
   - [ ] Complete implementation
 - [x] Script engine for loading and executing Lua files.
 - [ ] Plugin system to allow for creation of whatever game one desires
 - [ ] Plugin manager (like `go get` but for DragonMUD plugins)
 - [ ] MUD Server

## Future

 - [ ] Admin web interface with game building capabilities

# Contributing

Please reference CONTRIBUTING.md for details on becoming a contributor and/or
collaborator.

## Building From Source

To manage this project and ensure reproducible builds I chose to use the [glide](https://github.com/Masterminds/glide)
dependency manage for Go. What this means is that you'll need Glide to ensure
you get the same build that I do. *Please do not update any dependency without
explicit reasoning to defend the upgrade.* If required to install new
dependencies you can simply do `glide get`. This will add the dependency to `vendor/`
which should not be committed.

So the process to set up for contributions is to fork it, and then `go get` your
project:

```sh
go get github.com/myusername/dragon-mud
cd $GOPATH/src/github.com/myusername/dragon-mud
glide install
```

At this point, you can either rename `/myusername/` to `/bbuck/` or you can
symlink `$GOPATH/src/github.com/myusername/dragon-mud` to `$GOPATH/src/github.com/bbuck/dragon-mud`
to avoid having to rewrite import paths.

To build your project:

```sh
make install
```

And if `make` is not available**, then install with the following command:

```sh
go install github.com/bbuck/dragon-mud/cmd/...
```

** But keep in mind, `make` is used to engineer standard multi-step processes for
building/installing so it's highly advantageous to get a version for your OS
up and running to use in place of trying to do everything manually.

# Contributors

Brandon Buck [@bbuck](https://github.com/bbuck) <lordizuriel@gmail.com>

# License

Copyright 2016-2017 Brandon buck

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
