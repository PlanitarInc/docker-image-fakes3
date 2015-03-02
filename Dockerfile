FROM planitar/dev-ruby

USER root

RUN gem install fakes3

EXPOSE 4567

CMD fakes3 -r /s3 -p 4567
