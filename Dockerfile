FROM alpine:3.4

USER root

RUN apk add --no-cache ruby

ARG PLNTR_FAKES3_VERSION=1.0.0.pre.8

# Planitar's fork of fakes3 contains additional CORS configuration.
# There are 3 additional allowed headers:
#   Content-MD5, X-AMZ-Content-SHA256, X-AMZ-Date
RUN gem install --no-ri --no-rdoc plntr-fakes3 -v ${PLNTR_FAKES3_VERSION}

EXPOSE 4567

ENV HOSTNAME 172.17.0.1

CMD fakes3 -r /s3 -p 4567 -H ${HOSTNAME}
