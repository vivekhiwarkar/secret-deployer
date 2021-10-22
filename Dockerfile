FROM golang:1.16.7
LABEL PROJECT="secret-deployer"

COPY src /go/src/secret-deployer/
WORKDIR /go/src/secret-deployer/

RUN go get -d

RUN go install

RUN mkdir -p bin
RUN go build -o bin/

ENTRYPOINT ["/go/src/secret-deployer/bin/secret-deployer"]