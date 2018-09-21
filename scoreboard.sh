#!/bin/sh

FILENAME=MyKuromasuSolver.java
ENDPOINT=http://193.196.36.59:8080
LAST_ID_FILE=.lastid

show_help() {
cat <<EOF
Usage: ./scoreboard.sh show
       ./scoreboard.sh submit [nick name]
       ./scoreboard.sh status [SUBMISSION ID]
EOF
}



lastid=${2:-$(cat $LAST_ID_FILE)}

case $1 in
    submit)
        echo "Submitting result..."
        nickname=$2
        curl -X POST --upload-file $FILENAME $ENDPOINT/submission?nickname=$nickname 2>/dev/null | tee $LAST_ID_FILE
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
