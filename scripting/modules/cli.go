// Copyright 2016-2017 Brandon Buck

package modules

import (
	"fmt"
	"reflect"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/cobra"
)

// Cli is a module designed specifically for adding new commands to the dragon
// application.
//   commandInfo: table = {
//     name: string = name used to call the command
//     summary: string? = short description of the subcommand
//     description: string? = long description of the subcommand
//     run: function = function to run if the command is called
//     flags: table? {
//       {
//         type: string = "number" | "string" | "boolean"
//         name: string = name of the flag, if it's "thing" the flag is "--thing"
//         short: string = short description of the flag
//         description: string = long description of the flag
//         default: string | number | boolean = default value of the flag
//       }
//     }
//   }
//   add_command(cmd_info: commandInfo): boolean
//     add a subcommand based on the information provided. Allowing any plguin
//     to add their own commands.
var Cli = lua.TableMap{
	"add_command": func(eng *lua.Engine) int {
		cmdTbl := eng.PopTable()

		if !cmdTbl.IsTable() {
			eng.PushValue(eng.False())
			log("cli").Warn("{W}cli.add_command{x} was called without a table value")

			return 1
		}

		run := cmdTbl.Get("run")
		if !run.IsFunction() {
			eng.PushValue(eng.False())
			log("cli").Warn("No run command defined for the command.")

			return 1
		}

		cmd := new(cobra.Command)
		cmd.Use = cmdTbl.Get("name").AsString()
		if cmd.Use == "" {
			eng.PushValue(eng.False())
			log("cli").Warn("No name was provided for the command, a name is required.")

			return 1
		}

		cmd.Short = cmdTbl.Get("summary").AsString()
		cmd.Long = cmdTbl.Get("description").AsString()
		pflags := make(map[string]interface{})
		cmd.Run = func(_ *cobra.Command, args []string) {
			luaArgs := eng.TableFromSlice(args)
			flags := make(map[string]interface{})
			for k, ptr := range pflags {
				rval := reflect.ValueOf(ptr)
				if rval.IsNil() {
					flags[k] = nil
					continue
				}
				flags[k] = rval.Elem().Interface()
			}
			luaFlags := eng.TableFromMap(flags)
			_, err := run.Call(0, luaArgs, luaFlags)
			if err != nil {
				logger.NewWithSource(fmt.Sprintf("cmd(%s)", cmd.Use)).WithError(err).Fatal("Failed to execute lua command")
			}
		}

		cmdFlags := cmdTbl.Get("flags")
		if !cmdFlags.IsNil() {
			cmdFlags.ForEach(func(key *lua.Value, finfo *lua.Value) {
				name := finfo.Get("name").AsString()
				short := finfo.Get("short").AsString()
				typ := finfo.Get("type").AsString()
				desc := finfo.Get("description").AsString()
				def := finfo.Get("default").AsRaw()

				if name == "" || short == "" || typ == "" || desc == "" {
					log("cli").WithFields(logger.Fields{
						"name":        name,
						"short":       short,
						"type":        typ,
						"description": desc,
					}).Warn("name, short, type and description are required for flags to be defined")

					return
				}

				switch typ {
				case "string":
					s, _ := def.(string)
					str := cmd.Flags().StringP(name, short, s, desc)
					pflags[name] = str
				case "boolean":
					bval, _ := def.(bool)
					b := cmd.Flags().BoolP(name, short, bval, desc)
					pflags[name] = b
				case "number":
					f64, _ := def.(float64)
					f := cmd.Flags().Float64P(name, short, f64, desc)
					pflags[name] = f
				default:
					log("cli").WithField("type", typ).Warn("Type value is not valid.")
				}
			})
		}

		rootCmd := eng.Meta[keys.RootCmd].(*cobra.Command)
		rootCmd.AddCommand(cmd)

		eng.PushValue(eng.True())

		return 1
	},
}
