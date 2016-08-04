FROM golang:1.5.1

# SSL keys
RUN mkdir -p /code/.keys
ADD deployment/keys/ /code/.keys/

# Install the streamer
RUN mkdir -p /code/siphon-streamer
ADD src /code/siphon-streamer/src
ADD streamer /code/siphon-streamer/
RUN chmod +x /code/siphon-streamer/streamer

WORKDIR /code/siphon-streamer
ENTRYPOINT ["./streamer"]
