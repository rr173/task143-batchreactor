FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/batchreactor .

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app
COPY --from=build /out/batchreactor /app/batchreactor
ENTRYPOINT ["/app/batchreactor"]
CMD ["--addr=:8080"]
