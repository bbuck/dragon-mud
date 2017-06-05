// Copyright 2016-2017 Brandon Buck

package modules

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
//         default: string | number | boolean | string(duration) = default value of the flag
//       }
//     }
//   }
//   add_command(cmd_info): boolean
//     @param cmd_info: commandInfo = the information necessary to build out
//       a new command for the command line interface.
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
		cmd.Run = func(cmd *cobra.Command, args []string) {
			luaArgs := eng.TableFromSlice(args)
			flags := make(map[string]interface{})
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				fval := reflect.ValueOf(f.Value)
				fval = reflect.Indirect(fval)
				fname := strings.ToLower(f.Name)
				flags[fname] = fval.Interface()
			})
			luaFlags := eng.TableFromMap(flags)
			_, err := run.Call(0, luaArgs, luaFlags)
			if err != nil {
				logger.NewWithSource(fmt.Sprintf("cmd(%s)", cmd.Use)).WithError(err).Fatal("Failed to execute lua command")
			}
		}

		cmdFlags := cmdTbl.Get("flags")
		if !cmdFlags.IsNil() {
			for i := 1; i <= cmdFlags.Len(); i++ {
				finfo := cmdFlags.RawGet(i)
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

					continue
				}

				switch typ {
				case "string":
					s, _ := def.(string)
					cmd.Flags().StringP(name, short, s, desc)
				case "boolean":
					bval, _ := def.(bool)
					cmd.Flags().BoolP(name, short, bval, desc)
				case "number":
					f64, _ := def.(float64)
					cmd.Flags().Float64P(name, short, f64, desc)
				case "duration":
					d, _ := def.(string)
					dur, err := time.ParseDuration(d)
					if err != nil {
						eng.RaiseError(err.Error())

						return 0
					}
					cmd.Flags().DurationP(name, short, dur, desc)
				// TODO: Add more types
				default:
					log("cli").WithField("type", typ).Warn("Type value is not valid.")
				}
			}
		}

		rootCmd := eng.Meta[keys.RootCmd].(*cobra.Command)
		rootCmd.AddCommand(cmd)

		eng.PushValue(eng.True())

		return 1
	},
}
