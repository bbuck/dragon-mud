-- Commands the {{.Name}} plugin adds. Each has forms: patterns players
-- type, each with the function that runs.
return {
	["{{.Name}}"] = {
		desc = "Check that the {{.Name}} plugin is loaded.",
		forms = {
			{ "{{.Name}}", function(actor)
					actor:send("The {{.Name}} plugin is working.")
				end },
		},
	},
}
