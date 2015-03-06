FROM planitar/dev-ruby

USER root

RUN gem install fakes3

EXPOSE 4567

ENV HOSTNAME 172.17.42.1

CMD fakes3 -r /s3 -p 4567 -h $HOSTNAME
