FROM golang:1.16-buster as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o petkit-proxy main.go
#######################################
FROM scratch
WORKDIR /app
COPY --from=builder /app/petkit-proxy .
EXPOSE 8080
CMD ["./petkit-proxy"]
