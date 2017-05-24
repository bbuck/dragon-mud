package modules

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"runtime"

	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/pzduniak/argon2"
	"github.com/spf13/viper"
)

// Password provides a method to take options to hash a password using the argon2i
// encryption algorithm.
//   options table = {
//     salt : string = a string value used to differ password hashes
//     iteration : number = a number value used to determine the number of hash
//       iterations over the password to produce a hash
//   getRandomParams(): options
// 	   return a table with two keys, 'salt' and 'iterations' that have been
//     cyrptographically secure randomly generated for use with hash and isValid.
//   hash(password, options): string
//     @param password: string = a plaintext password value
//     @param options: options = the options values with which to encrypt the
//       password by
//     hashes the plain text password using the argon2i algorith with the data
//     in the provided table. The table must have a 'salt' and 'iterations'
//     field.
//   isValid(password: string, hash: string, options: table): string
//     @param password string = the plain text password entered by the user that
//       will be compared against the hash
//     @param hash: string = a hash of an encrypted password that the new
//       password should match after encryption
//     @param options: options = the options values with which to decrypt the
//       password by
//     hashes the password using the options given and compares the output hash
//     to the given hash, true means the given password matches the hash
var Password = lua.TableMap{
	"getRandomParams": func(engine *lua.Engine) int {
		num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			log("password").WithError(err).Error("Failed to generate secure salt, CEASE OPERATION IMMEDIATELY")
			engine.RaiseError(err.Error())

			return 0
		}
		salt := fmt.Sprintf("%x", num)
		minIterations := viper.GetInt("crypto.min_iterations")
		if minIterations <= 0 {
			minIterations = 1
		}
		maxIterations := viper.GetInt("crypto.max_iterations")
		if maxIterations <= minIterations {
			maxIterations = minIterations + 1
		}
		iterations := uint32(random.Range(minIterations, maxIterations))

		table := engine.NewTable()
		table.Set("salt", salt)
		table.Set("iterations", iterations)

		engine.PushValue(table)

		return 1
	},

	"hash": func(engine *lua.Engine) int {
		table := engine.PopTable()
		password := engine.PopString()

		saltStr := table.Get("salt").AsString()
		iterations := uint32(table.Get("iterations").AsNumber())
		hash, err := hashPassword(password, saltStr, iterations)
		if err != nil {
			log("password").WithError(err).Error("Failed to hash password via Argon2i, CEASE OPERATION IMMEDIATLEY")
			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(string(hash))

		return 1
	},

	"isValid": func(engine *lua.Engine) int {
		params := engine.PopTable()
		hashed := engine.PopString()
		password := engine.PopString()

		saltStr := params.Get("salt").AsString()
		iterations := uint32(params.Get("iterations").AsNumber())
		hash, err := hashPassword(password, saltStr, iterations)
		if err != nil {
			log("password").WithError(err).Error("Failed to validate password, CEASE OPERATION IMMEDIATLEY")
			engine.PushValue(engine.False())

			return 1
		}

		engine.PushValue(hashed == hash)

		return 1
	},
}

func hashPassword(password, saltStr string, iterations uint32) (string, error) {
	memSize := viper.GetInt64("crypto.password_memory_size")
	length := viper.GetInt64("crypto.password_length")
	pass := []byte(password)
	salt := []byte(saltStr)
	hash, err := argon2.Key(pass, salt, iterations, uint32(runtime.NumCPU()), uint32(memSize), int(length), argon2.Argon2i)

	return string(hash), err
}
