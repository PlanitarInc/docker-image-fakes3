FROM planitar/dev-ruby

USER root

# Planitar's fork of fakes3 contains additional CORS configuration.
# There are 3 additional allowed headers:
#   Content-MD5, X-AMZ-Content-SHA256, X-AMZ-Date
RUN gem install plntr-fakes3 -v 1.0.0.pre.7

EXPOSE 4567

ENV HOSTNAME 172.17.0.1

CMD fakes3 -r /s3 -p 4567 -H $HOSTNAME
