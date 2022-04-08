FROM golang:1.18.0 as build

WORKDIR /app

COPY go.* /app/

COPY . .

RUN go build -o /app/kubeSecretSync

# Need glibc
FROM gcr.io/distroless/base
COPY --from=build /app/kubeSecretSync /app/

ENTRYPOINT [ "/app/kubeSecretSync" ]
