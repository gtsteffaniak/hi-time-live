FROM golang:1.22-alpine
WORKDIR /app
COPY ["./","./"]
WORKDIR /app/backend
RUN go build .

FROM alpine
COPY --from=0 ["/app/backend/hi-time-live","/app/frontend","./"] 
CMD ["./hi-time-live"]