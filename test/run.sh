sleep $(shuf -i 1-3 -n 1)
/usr/bin/time -f 'user=%U sys=%S real=%e' g++ -o main main.cpp
/usr/bin/time -f 'user=%U sys=%S real=%e' ./main
echo $1score=$(shuf -i 100-10000 -n 1)
