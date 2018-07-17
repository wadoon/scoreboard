# scoreboard

A small web-service for scoreboard of student assignments



# Create a new assignment

Add a new record in `config.js` with following values:

```js
[ ...,
 {
    "boardFileName": "board.json", // path to the file to store the assignments
    "script": "test/run.sh",       // shell script, that evaluates the assignment
    "title": "Test Instance",      // title
    "description": "Testing purpose", // description
    "templateFile": "test/template",  // path to a file, which stores the preamble of the scoreboard
    "endpoint": "/test",              // URL
    "submissionFilename": "main.cpp", // Filename to store the sent submission content.
    "submissionFolder": "test/submission/" //Folder to store the submissions
  }
]
```

Privilege escape is needed in the given `script file`, e. g.
avoid escaping the current folder (chroot).

# Endpoints

## GET `/<endpoint>`

Returns the current scoreboard. Service accepts a paramter `ids` (comma-separated list of values)
and prints out their rank, too.

### Example

```
$ curl -X GET localhost:8080/test?ids=365a858149c6e2d1,2e3108dabb158644
Test Instance

Testing purpose
 #  NAME                  PTS   TIME DATE
--------------------------------------------------------------------------------
001 1a02070f169c1121     1001 61.730 2018-07-17T07:04:55+02:00
002 2e3108dabb158644     1000  0.084 2018-07-17T07:04:28+02:00
003 6e661e92759805f5       99  1.088 2018-07-17T07:04:50+02:00
004 c90bd268b68e6a3f       99  1.193 2018-07-17T07:04:35+02:00


--------------------------------------------------------------------------------
Submission 365a858149c6e2d1 not found!
Your submission 2e3108dabb158644 on rank 1!

================================================================================
Server version: 1.0	Server time: 2018-07-17T16:42:35+02:00
https://github.com/wadoon/scoreboard	Alexander Weigl <weigl@kit.edu>%                                                                                                     ~/w/g/s/g/w/scoreboard %                                                                                                                              git:master* / 16:42:35
```


## POST `/<endpoint>`

Endpoint is for submission. It takes the submission as the post body and does the following steps:

1. Generates a random submission id.
2. Creates a temporal folder in `submissionFolder`.
3. Stores the submission into the `<submissionFolder>/<submissionFilename>`.
4. Executes the given `script` in the temporal folder.
   The script should terminate with `0` if this submission is valid (error free, etc.) and
   print out `<salt>score=<value>`. The salt is given as `$1` and prevents that the submission prints a score.
   Escalation needs to be prevented by the script.
5. Extracts the score from the script output.
6. Add valid submission to scoreboard and store the board.

### Example
```
curl -X POST --data @large.cpp localhost:8080/test
Your submission id is 4d65822107fcfd52
Environment is set up.
**************************************************************************
**************************   stdout/stderr   *****************************
main.cpp:1:1: Fehler: verirrtes »@« im Programm
 @large.cpp
 ^
main.cpp:1:2: Fehler: »large« bezeichnet keinen Typ
 @large.cpp
  ^~~~~
Command exited with non-zero status 1
user=0.00 sys=0.00 real=0.01
/usr/bin/time: cannot run ./main: No such file or directory
Command exited with non-zero status 127
user=0.00 sys=0.00 real=0.00
score=2519

**************************************************************************
Process exited with 0
Runtime: user/system/real =  0.01 /  0.01 /  0.03
Your position is 3.
```
