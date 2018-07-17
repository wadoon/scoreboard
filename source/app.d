import vibe.appmain;
import vibe.http.server;
import vibe.http.router;
import vibe.core.log;


import std.algorithm;
import std.algorithm.sorting;
import std.array;
import std.csv;
import std.file;
import std.stdio;


/*
  // (add-to-list 'exec-path (expand-file-name "~/dlang/dmd-2.079.0/linux/bin64/dmd"))
*/

//global data
const LEADERBOARD_FILE = "leaderboard.csv";
const LEADERBOARD_FILE_TMP = "leaderboard.csv~";

struct Record {
    int id = 0;
    string name = "";
    int score = 0;
    int runtime = 99999;
    string date = "n/a";

    int opCmp(ref const Record o) const {
        auto fields = [ o.score.opCmp(score), //higher -> better
                        runtime.opCmp(o.runtime),//smaller -> better
                        o.date.opCmp(date)]; // first wins

        foreach(v; fields) {
            if(v!=0) return v;
        }
        return 0;
    }
}

synchronized class Global {
    private Record[] currentBoard;

    synchronized void  add(Record r) {
        currentBoard ~= r;
        currentBoard.sort();
        auto f = File(LEADERBOARD_FILE_TMP, "w"); // open for writing
        foreach(r; currentBoard) {
            f.write(r.id);
            f.write(',');
            f.write(r.name);
            f.write(',');
            f.write(r.score);
            f.write(',');
            f.write(r.runtime);
            f.write(',');
            f.write(r.date);
            f.writeln();
        }
        f.close();
        std.file.rename(LEADERBOARD_FILE_TMP, LEADERBOARD_FILE);
    }

    synchronized void  load() {
        currentBoard = File(LEADERBOARD_FILE, "r")
            .byLineCopy(false)
            .map!(split!',')
            .map!(a => Record(a[0].to!uint, a[1],
                              a[2].to!uint,
                              a[3].to!uint, a[4]));
    }
}

shared Global global;
//


void index(HTTPServerRequest req, HTTPServerResponse res)
{
    res.writeBody("Hello, World!");
}

void submit(HTTPServerRequest req, HTTPServerResponse res)
{
    res.writeBody("Hello, World!");
}

void errorPage(HTTPServerRequest req, HTTPServerResponse res,
               HTTPServerErrorInfo error)
{
    res.writeBody("error");
}

shared static this()
{
    auto router = new URLRouter;

    router.get("/", &index);
    router.post("/", &submit);

    auto settings = new HTTPServerSettings;
    settings.port = 8080;
    settings.errorPageHandler = toDelegate(&errorPage);
    settings.bindAddresses = ["::1", "127.0.0.1"];

    listenHTTP(settings, router);

    logInfo("Please open http://127.0.0.1:8080/ in your browser.");
    //runApplication();
}

