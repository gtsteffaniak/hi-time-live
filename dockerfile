FROM golang:1.22-alpine
WORKDIR /app
COPY ["./","./"]
WORKDIR /app/site
RUN go build -ldflags='-w -s' .

FROM scratch
WORKDIR /app
COPY --from=0 ["/app/site/hi-time-live","./"]
COPY ["site/templates","./templates"]
COPY ["site/static","./static"]
CMD ["./hi-time-live"]