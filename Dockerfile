FROM planitar/dev-base

USER root

RUN apt-get install -y ruby ruby-dev && apt-get clean
RUN gem install fakes3

VOLUME s3
EXPOSE 4567

CMD fakes3 -r /s3 -p 4567
