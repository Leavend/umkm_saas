# Placeholder Dockerfile (API)
FROM golang:1.22 as build
WORKDIR /app
COPY . .
# RUN go mod download && go build -o /out/api ./cmd/api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
# COPY --from=build /out/api /app/api
# EXPOSE 8080
# ENTRYPOINT ["/app/api"]
