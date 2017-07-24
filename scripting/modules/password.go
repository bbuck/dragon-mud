package modules

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"
)

// Password provides a method to take options to hash a password using the argon2i
// encryption algorithm.
//   hash(password): string
//     @param password: string = a plaintext password value
//     hashes the plain text password using the bcrypt algorithm for hasing
//     passwords and the configured cost in Dragonfile.toml.
//   is_valid(password: string, hash: string): string
//     @param password string = the plain text password entered by the user that
//       will be compared against the hash
//     @param hash: string = a hash of an encrypted password that the new
//       password should match after encryption
//     hashes the given password and compares it to the hashed password (using
//     the same cost that the hashed password was generated with) and compares
//     the result.
var Password = lua.TableMap{
	// hash the given string password using bcrypt
	"hash": func(engine *lua.Engine) int {
		passwordArg := engine.PopValue()
		if !passwordArg.IsString() {
			engine.PushValue(nil)

			return 1
		}
		password := passwordArg.AsString()

		cost := getBcryptCost()
		res, err := bcrypt.GenerateFromPassword([]byte(password), cost)
		if err != nil {
			engine.PushValue(nil)

			return 1
		}

		engine.PushValue(string(res))

		return 1
	},
	// compares the given password to the given hash (after hashing the password
	// with the same options as the given hash)
	"is_valid": func(engine *lua.Engine) int {
		if engine.StackSize() < 2 {
			engine.PushValue(false)

			return 1
		}

		hashedArg := engine.PopValue()
		passwordArg := engine.PopValue()
		if !hashedArg.IsString() || !passwordArg.IsString() {
			engine.PushValue(false)

			return 1
		}

		hashed := hashedArg.AsString()
		password := passwordArg.AsString()

		err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))

		engine.PushValue(err == nil)

		return 1
	},
}

// return an integer cost value based on the project configuration
func getBcryptCost() int {
	rawCost := viper.Get("crypto.cost")
	if costInt, ok := rawCost.(int); ok && costInt >= bcrypt.MinCost && costInt <= bcrypt.MaxCost {
		return costInt
	}

	return random.Range(bcrypt.DefaultCost, bcrypt.MaxCost)
}
