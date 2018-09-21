FROM alpine

ADD config.json .
ADD scoreboard  .
ADD kuromasu    .
ADD test        .

CMD ./scoreboard
