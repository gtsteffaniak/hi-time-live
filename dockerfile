FROM golang:1.22-alpine
WORKDIR /app
COPY ["./","./"]
WORKDIR /app/site
RUN go build .

FROM alpine
WORKDIR /app
COPY --from=0 ["/app/site/hi-time-live","./"] 
COPY --from=0 ["/app/site/templates/*","./templates"] 
CMD ["./hi-time-live"]