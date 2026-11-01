FROM golang:1.27.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG MAIN_PACKAGE=.
RUN CGO_ENABLED=0 go build -o /app ${MAIN_PACKAGE}

FROM gcr.io/distroless/static-debian12
COPY --from=build /app /app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app"]
