package modules

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"runtime"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/random"
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/pzduniak/argon2"
	"github.com/spf13/viper"
)

var passwordLog = logger.LogWithSource("lua password")

// Password provides a method to take options to hash a password using the argon2i
// encryption algorithm.
var Password = map[string]interface{}{
	"GetRandomParams": func(engine *engine.Lua) int {
		num, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		successful := engine.True()
		if err != nil {
			passwordLog.WithField("error", err.Error()).Error("Failed to generate secure salt, CEASE OPERATION IMMEDIATELY")
			successful = engine.False()
			num = big.NewInt(0)
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
		table.Set("Salt", salt)
		table.Set("Iterations", iterations)

		engine.PushValue(table)
		engine.PushValue(successful)

		return 2
	},

	"Hash": func(engine *engine.Lua) int {
		table := engine.PopTable()
		password := engine.PopString()

		saltStr := table.Get("Salt").AsString()
		iterations := uint32(table.Get("Iterations").AsNumber())
		hash, err := hashPassword(password, saltStr, iterations)
		if err != nil {
			passwordLog.WithField("error", err.Error()).Error("Failed to hash password via Argon2i, CEASE OPERATION IMMEDIATLEY")
			engine.PushValue("")
			engine.PushValue(engine.False())

			return 2
		}

		engine.PushValue(string(hash))
		engine.PushValue(engine.True())

		return 2
	},

	"IsValid": func(engine *engine.Lua) int {
		params := engine.PopTable()
		hashed := engine.PopString()
		password := engine.PopString()

		saltStr := params.Get("Salt").AsString()
		iterations := uint32(params.Get("Iterations").AsNumber())
		hash, err := hashPassword(password, saltStr, iterations)
		if err != nil {
			passwordLog.WithField("error", err.Error()).Error("Failed to validate password, CEASE OPERATION IMMEDIATLEY")
			engine.PushValue(engine.False())

			return 1
		}

		if hashed != hash {
			engine.PushValue(engine.False())

			return 1
		}

		engine.PushValue(engine.True())

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
