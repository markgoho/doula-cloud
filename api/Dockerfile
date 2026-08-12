FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /app

FROM gcr.io/distroless/static-debian12
COPY --from=build /app /app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app"]
