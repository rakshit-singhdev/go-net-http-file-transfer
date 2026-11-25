# go-net-http-file-transfer

## API docs (Swagger)

With the server running, open `http://localhost:8080/swagger/index.html`.

After changing handler annotations, regenerate the spec:

```sh
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g main.go -o docs
```
