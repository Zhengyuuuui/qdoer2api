FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/qoder2api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/qoder2api /app/qoder2api
EXPOSE 3588 8963
ENV HOME=/data
VOLUME ["/data"]
CMD ["/app/qoder2api", "--bind=0.0.0.0", "--web-port=3588", "--bridge-port=8963"]
