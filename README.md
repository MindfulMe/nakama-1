# Nakama

## Instructions

Install [CockroachDB](https://www.cockroachlabs.com/), [Git](https://git-scm.com/) and [Go](https://golang.org/).
The install it:
```bash
go get -u github.com/nicolasparada/nakama
```

For your knowledge, it uses the following Go dependencies:
- `github.com/lib/pq`
- `github.com/go-chi/chi`
- `github.com/dgrijalva/jwt-go`
- `github.com/cockroachdb/cockroach-go/crdb`
- `github.com/gernest/mention`

Then start the database and create the schema:
```bash
cockroach start --insecure --host 127.0.0.1
cat schema.sql | cockroach sql --insecure
```

You will need an SMTP server for the passwordless authentication. I recommend you [mailtrap.io](https://mailtrap.io/) to test it.

Set `SMTP_USERNAME` and `SMTP_PASSWORD` as environment variables.

Build and run:
```
go build
./nakama
```

`main.go` contains the route definitions; check those.
