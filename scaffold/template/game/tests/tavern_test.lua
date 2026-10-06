-- Tests for your game. Run them with `dragon test`.
--
-- Every file in tests/ whose name ends in _test.lua returns a table of
-- tests. Each test gets a fresh copy of the game, with its plugins and an
-- empty world, and plays it the way players do: t:connect() opens a
-- connection, p:login(name) makes an account, p:send(line) types, and
-- p:expect(text) waits for output containing text. Run only some tests
-- with `dragon test -run tavern`.
--
-- t:eval(code) runs Lua inside the game, as if it were in lua/, to set up
-- the world or look at it, and returns what it returns, with objects as
-- their ids.

return {
	["new players arrive in the tavern"] = function(t)
		local alice = t:connect()
		alice:login("Alice")
		alice:expect("The barkeep looks up and nods at you.")
		alice:expect("The Dragon's Rest")
	end,

	["others see you dance"] = function(t)
		local alice, bob = t:connect(), t:connect()
		alice:login("Alice")
		bob:login("Bob")
		alice:expect("Bob has arrived.")

		bob:send("dance")
		bob:expect("You dance a little jig.")
		alice:expect("Bob dances a little jig.")
	end,

	["the tavern is made once"] = function(t)
		local name = t:eval([[ return require("dragon.world").keyed("tavern"):get("name") ]])
		assert(name == "The Dragon's Rest", "the tavern is called " .. tostring(name))
	end,
}
