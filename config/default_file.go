package config

// Contents is the data necessary for a basic DragonDetails.toml file, it will
// be created when a dragon init command is run.
const Contents = `# DragonDetails base file

# Game network configuration.
[net]

# Change the port the game runs on, by default this is 8080
# game_port = 8080
# Change the port the server private port runs on. This private port is only
# connectable from the machine hosting the server and is the only way to send
# 'SERVER' messages to all memebers of the game. Other than this, it's the same
# as connecting to the standard port.
# private_port = 8081

# Define log targets
# Log type consist of 'os' and 'file'. The 'os' types can have a target of
# 'stdout' and 'stderr' while the target of the 'file' types should be either a
# relative path to the log file or absolute path. Multiple log targets are
# supported.
[[log.targets]]

type = "os"
target = "stdout"

# Configure the database. For a list of supported databases please check the
# wiki. The default database is SQLite.
[database]

adapter = "sqlite3"
file = "data/game.sqlite3"

# adapter = "postgres"
# user = ""
# dbname = "dragon_mud"
# sslmode = "disable"
# dsn = "postgres://user:password@localhost/dragon_mud?sslmode=disable"

# adapter = "mysql"
# dsn = "user:password@]localhost:3306/dragon_mud"
`
