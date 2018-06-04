FROM golang:1.8

RUN apt-get update
RUN apt-get install -y autoconf


ENV GOPATH /gopath
ENV ROUTER  ${GOPATH}/src/github.com/emphant/redis-mulive-router
ENV PATH   ${GOPATH}/bin:${PATH}:${CODIS}/bin
COPY . ${ROUTER}


RUN make -C ${ROUTER} distclean
RUN make -C ${ROUTER} build
RUN make -C ${ROUTER} install
RUN rm -rf ${ROUTER}/*

CMD ["rds-router"]

