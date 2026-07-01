package fees

TemporalHostPort: [
	if #Meta.Environment.Cloud == "local" { "localhost:7233" },
	"temporal:7233",
][0]

SQLitePath: [
	if #Meta.Environment.Cloud == "local" { "storage/database/gocanto.sqlite3" },
	"/tmp/gocanto/gocanto.sqlite3",
][0]
