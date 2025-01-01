FROM golang:1.22 as build-env
WORKDIR /go/src/github.com/ayinke-llc/sdump

COPY ./go.mod /go/src/github.com/ayinke-llc/sdump
COPY ./go.sum /go/src/github.com/ayinke-llc/sdump

RUN go mod download && go mod verify
COPY . .

ENV CGO_ENABLED=0 
RUN go install ./cmd

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/cmd /
CMD ["/cmd","http"]

