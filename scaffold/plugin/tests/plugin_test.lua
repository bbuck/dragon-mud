-- Tests for the {{.Name}} plugin. Run them with dragon test, or just these
-- with dragon test -run {{.Name}}.
return {
	["the {{.Name}} plugin is loaded"] = function(t)
		local player = t:connect()
		player:login("Tester")
		player:send("{{.Name}}")
		player:expect("The {{.Name}} plugin is working.")
	end,
}
