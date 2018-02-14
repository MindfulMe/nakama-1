# Nakama

## Instructions

Install [CockroachDB](https://www.cockroachlabs.com/), [Git](https://git-scm.com/) and [Go](https://golang.org/).
Then install the _go_ dependencies:
```bash
go get -u github.com/lib/pq
go get -u github.com/go-chi/chi
go get -u github.com/dgrijalva/jwt-go
go get -u github.com/cockroachdb/cockroach-go/crdb
go get -u github.com/gernest/mention
```

Start the database and create the schema:
```bash
cockroach start --insecure --host 127.0.0.1
cat schema.sql | cockroach sql --insecure
```

You will need an SMTP server for the passwordless authentication. I recommend you [mailtrap.io](https://mailtrap.io/) to test it.
Create and account and inbox there and save your credentials.

Build and run:
```
go build
./nakama -smtpuser your_mailtrap_username -smtppwd your_mailtrap_password
```

`main.go` contains the route definitions; check those.
