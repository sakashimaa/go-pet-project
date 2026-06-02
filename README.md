Goose installation:
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Migrations:
```bash
make migrate-up-<microservice_folder_name>
```
Example:
```bash
make migrate-up-auth
```