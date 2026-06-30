package fees

TemporalHostPort: [
	if #Meta.Environment.Cloud == "local" { "localhost:7233" },
	"temporal:7233",
][0]

