B=/home/egc/IEC-SIM/tsdb-build
R=/home/egc/IEC-SIM/tsdb-run
# stop only OUR test process (matches our own run dir)
pkill -f "$R/bin/gridsim" 2>/dev/null; sleep 1
rm -rf $R && mkdir -p $R/bin $R/logs $R/web
cp $B/gridsim-bin $R/bin/gridsim
cp -r $B/web/dist $R/web/dist
cp -r $B/config $R/config
cd $R
nohup $R/bin/gridsim serve --http ":18989" --config-dir $R/config --log-dir $R/logs --log info >> $R/logs/output.log 2>&1 &
echo $! > $R/logs/pid
sleep 4
echo "pid=$(cat $R/logs/pid)"; ps -p $(cat $R/logs/pid) -o pid,cmd | tail -1
tail -15 $R/logs/output.log
echo "== status"; curl -s -m 5 -i http://127.0.0.1:18989/api/v1/status | head -12
echo "== instances.json"; head -c 600 $R/config/instances.json
echo; ls $R/config/py_simulator | head; ls $R/config/py_simulator/config 2>/dev/null | head -20
echo "== users"; ls $R/config/users.json 2>&1
