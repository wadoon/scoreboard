#!/bin/sh

FILENAME=large.cpp
ENDPOINT=localhost:8080/test
LAST_ID_FILE=.lastid

show_help() {
cat <<EOF
Usage: ./scoreboard.sh show
       ./scoreboard.sh submit
       ./scoreboard.sh status [SUBMISSION ID]
EOF
}



lastid=${2:-$(cat $LAST_ID_FILE)}

case $1 in
    submit)
        echo "Submitting result..."
        curl -X POST --data @$FILENAME $ENDPOINT/submission 2>/dev/null | tee $LAST_ID_FILE
        lastid=$(grep "Your submission id is" $LAST_ID_FILE | cut -d' ' -f 6)
        echo $lastid >$LAST_ID_FILE
        ;;
    status)
        curl -X GET $ENDPOINT/submission/$lastid
        ;;
    show)
        curl -X GET $ENDPOINT/?ids=${lastid}
        ;;
    *)
        show_help; exit 1
        ;;
esac
