# DragonMUD

[![Build Status](https://travis-ci.org/bbuck/dragon-mud.svg?branch=develop)](https://travis-ci.org/bbuck/dragon-mud)
[![Code Climate](https://codeclimate.com/github/bbuck/dragon-mud/badges/gpa.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Issue Count](https://codeclimate.com/github/bbuck/dragon-mud/badges/issue_count.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Discord Channel](https://img.shields.io/badge/discord-DragonMUD-blue.svg?style=flat)](https://discord.gg/78TCMuq)
[![Go Report Card](https://goreportcard.com/badge/github.com/bbuck/dragon-mud)](https://goreportcard.com/report/github.com/bbuck/dragon-mud)

DragonMUD is a dream of mine, building a new MUD engine for experience and fun
before building my own game on top of it. The engine will be a firm foundation
for any kind of text based adventure (for now just Telnet, but eventually web
and websocket versions as well). It will feature a plugin system allowing the
core of the game to be moldable from the ground up into what you truly desire
for your game.

### Why should I use this?

That's really up to you. This project is for me, but I believe in sharing. I
also feel yet another new MUD engine may inspire some new games to be created
in the genre, which would be amazing. I'm a huge fan of MUDs and feel that new
entries have kind of become almost non-existent. Perhaps a new game and an
accessible low-setup server framework would make it easier for new games to be
quickly created.

### What exactly is it?

DragonMUD started life as an engine designed specifically for the MUD that I
wanted to build, but before long it pivoted to being a base for text based
multi-user games. Leverage the power of Lua running on top of Go, DragonMUD
aims to provide a quick entry into the genre. Once you generate a new game you
can immediately start adding plugins that the community has put together to
piece your game together. Want classic rooms? Grab a plugin, what geographically
organized rooms? Grab a plugin, want a mapping system? Grab a plugin -- if you
can't find one, the build your own and share it with the community!

DragonMUD provides the foundation and the glue code to allow many different
plugins to come together and form a cohesive bond, allowing you to build the
text based game of your dreams (or non-game, if that's your thing).

### What exactly is it not?

DragonMUD is engineered specifically for text based games running over TELNET.
It doens't have to be games though, but it does need to function over TELNET.
There are plans to provide an HTTP server with websocket support in the future
but for the time being you should probably find another engine if you're
looking to do anything that isn't text-based.

## Why Go?

I love C and C++ but they're older and slightly more complex languages to set
up and maintain. [Go](https://golang.org/) strives to be a "modern" C and was
therefore a good choice in my opinion. It also supports concurrency out of the
box in a very easy to use and understand way. It's also relatively low level
enough for the purpose of a running a game server.

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
 - [x] Test suite to validate working state of features, capturing as many
   differing/edge cases as possible
 - [ ] Neo4j backed database features
   - [x] Neo4j database connection library available to Lua (in development)
   - [ ] ActiveRecord-esque Entity framework for the scripts to leverage
 - [x] Script engine for loading and executing Lua files.
 - [ ] Plugin system to allow for creation of whatever game one desires
 - [ ] Plugin manager (like `go get` but for DragonMUD plugins)
 - [ ] Telnet Server

## Future

 - [ ] Web layer for defining and building any kind of web server on top of
   the chosen application

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
go get github.com/bbuck/dragon-mud
cd $GOPATH/src/github.com/bbuck/dragon-mud
make get-glide
make get-deps
```

To build your project:

```sh
make install
```

And if `make` is not available**, then install with the following command:

```sh
glide install
go get github.com/onsi/ginkgo/ginkgo
go get github.com/jteeuwen/go-bindata/...
go-bindata -pkg assets -o assets/assets.go -prefix assets/raw assets/raw/...
go install github.com/bbuck/dragon-mud/cmd/...
```

\*\* But keep in mind, `make` is used to engineer standard multi-step processes for
building/installing so it's highly advantageous to get a version for your OS
up and running to use in place of trying to do everything manually.

# Contributors

Brandon Buck, [@bbuck](https://github.com/bbuck), <lordizuriel@gmail.com>

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
