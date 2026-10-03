FROM golang:1.27.1
# Dependencies are prepared explicitly at image build time, never during a review.
COPY go.mod go.sum /opt/dependencies/
RUN cd /opt/dependencies && go mod download && chmod -R a+rX /go/pkg/mod
COPY run-tests.sh /giad-test
RUN chmod 0555 /giad-test
ENV GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOFLAGS=-buildvcs=false
