# DragonMUD

[![Issues In Progress](https://badge.waffle.io/bbuck/dragon-mud.svg?label=in%20progress&title=In%20Progress)](http://waffle.io/bbuck/dragon-mud)
[![Build Status](https://travis-ci.org/bbuck/dragon-mud.svg?branch=master)](https://travis-ci.org/bbuck/dragon-mud)
[![Code Climate](https://codeclimate.com/github/bbuck/dragon-mud/badges/gpa.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Issue Count](https://codeclimate.com/github/bbuck/dragon-mud/badges/issue_count.svg)](https://codeclimate.com/github/bbuck/dragon-mud)
[![Coverage Status](https://coveralls.io/repos/github/bbuck/dragon-mud/badge.svg?branch=master)](https://coveralls.io/github/bbuck/dragon-mud?branch=master)
[![Discord Channel](https://img.shields.io/badge/discord-DragonMUD-blue.svg?style=flat)](https://discordapp.com/channels/141274099262423040/141274099262423040)


DragonMUD is a dream of mine, building a new MUD engine for experience and fun
before building my own game on top of it. The engine will have a very opinionated
idea of game rules (such as how skills works, etc.) but outside of game rules
will be entirely customizable through scripts (using Lua) and being open source
would be open to forks with custom game rules implemented.

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

 - [ ] MUD Server
 - [ ] Database backed server, can choose which database (default SQLite)
 - [ ] Admin web interface with game building capabilities
 - [ ] Scriptable server hooks using Lua files
 - [ ] In game scripts written in Lua
 - [ ] Test suites with extensive coverage (protected with Coveralls)
 - [ ] Code climate monitoring GPA of code, maintaining a A - B grade for overall
   project
 - [ ] TravisCI integration to easily demonstrate stable builds
 
# Contributing

If you wish to work on this project with me, fork it, choose a task and do your
work for that task before making a pull request. I will be using the [GitFlow](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow)
methodology for maintaining this repository so keep your work out of `master`
and `develop` but target `develop` for merging.

I will reject any pull request (no matter how awesome) that aren't done in a
'feature branch.' Sorry to be that guy, but I want to keep this project clean
and I want the master branch to represent the latest stable codebase.

Do all of your work to resolve an issue on the original repository, if an issue
doesn't exist for what you want to do then create one for it. If I'm unsure of
the necessity for you're work we'll discuss it on the issue and go from there.

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
go install github.com/bbuck/dragon-mud/cmd/...
```

# License

Copyright 2016 Brandon Buck

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
