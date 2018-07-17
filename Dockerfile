FROM alpine

ADD config.json .
ADD leaderboard .

CMD ./leaderboard