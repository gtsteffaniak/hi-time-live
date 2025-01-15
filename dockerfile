FROM golang:1.22-alpine
ARG VERSION
WORKDIR /app
COPY ["./","./"]
WORKDIR /app/
RUN go build -ldflags="-w -s -X 'github.com/gtsteffaniak/hi-time-live/routes.Version=${VERSION}'" .

FROM scratch
WORKDIR /app
COPY --from=0 ["/app/hi-time-live","./"]
CMD ["./hi-time-live"]