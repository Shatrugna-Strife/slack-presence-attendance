FROM ubuntu:latest

WORKDIR /

COPY ./slack-user-attendence-app /slack-user-attendence-app

RUN chmod 777 /slack-user-attendence-app

RUN apt update
RUN apt install -y ca-certificates

ENTRYPOINT ["/slack-user-attendence-app"]
