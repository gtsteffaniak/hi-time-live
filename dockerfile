FROM golang:1.22-alpine
WORKDIR /app
COPY ["./","./"]
WORKDIR /app/site
RUN go build .

FROM alpine
COPY --from=0 ["/app/site/hi-time-live","./"] 
CMD ["./hi-time-live"]