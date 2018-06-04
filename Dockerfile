FROM golang:1.9

RUN apt-get update
RUN apt-get install -y autoconf

ENV GOPATH /gopath
ENV RDSMU  ${GOPATH}/src/github.com/emphant/redis-mulive-router
ENV PATH   ${GOPATH}/bin:${PATH}:${RDSMU}/bin
COPY . ${RDSMU}

RUN make -C ${RDSMU} distclean
RUN make -C ${RDSMU} build

WORKDIR /codis
