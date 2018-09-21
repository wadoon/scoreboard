#!/bin/sh -x
scriptDir=$(readlink -f $(pwd)/../../)

export CLASSPATH=.:$scriptDir/kuromasu-1.3-deps.jar
export RIDDLES_FOLDER=$scriptDir/riddles
export KUROMASU_TIMEOUT=60
#export KUROMASU_NO_CHECK=false
JAVAARGS="-Xms6g -Xms4g -cp $CLASSPATH -Djava.security.manager -Djava.security.policy=$scriptDir/default.policy"

# compiling
rm MyKuromasuSolver.class
javac -cp $CLASSPATH MyKuromasuSolver.java 2>compile.log \
    || javac -encoding cp1252 -cp $CLASSPATH MyKuromasuSolver.java 2>compile.log \
    || ( echo "Could not compile solution in $(pwd)";
         cat compile.log;
         echo "score${1}=0"
         exit 1) \
    && java $JAVAARGS edu.kit.iti.formal.kuromasu.KuromasuTest | tee output.txt

score=$(grep -E '.+ of .+ successful!' output.txt | cut -d' ' -f 4)
echo score${1}=$score
