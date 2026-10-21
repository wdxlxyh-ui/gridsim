R=/home/egc/IEC-SIM/tsdb-run
A=http://127.0.0.1:18989/api/v1
pkill -f "$R/bin/gridsim" 2>/dev/null; sleep 1
echo '[]' > $R/config/instances.json
cd $R
nohup $R/bin/gridsim serve --http ":18989" --config-dir $R/config --log-dir $R/logs --log info > $R/logs/output.log 2>&1 &
echo $! > $R/logs/pid; sleep 3
echo "== status"; curl -s -m 5 -o /dev/null -w "%{http_code}\n" $A/status
echo "== create instance"
curl -s -m 10 -X POST $A/instances -H 'Content-Type: application/json' -d '{"name":"tsdbtest","protocol":"modbus_bridge","iec104_port":0,"xlsx_file":"","http_enabled":false,"modbus_bridge_config":{"script_dir":"py_simulator","modbus_port":15021,"poll_interval_ms":1000,"start_time":"08:00","device_json":"config/device.json"}}' | head -c 600; echo
ID=$(python3 -c "import json;d=json.load(open('$R/config/instances.json'));print(d[0]['id'])")
echo "ID=$ID"; echo $ID > $R/logs/test_id
ls $R/config/py_instances 2>&1 | head -3
echo "== T1 GET default tsdb-config"; curl -s -m 5 -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-config
echo "== T2 PUT config"
curl -s -m 5 -X PUT -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-config -H 'Content-Type: application/json' -d '{"enos":{"apigw_address":"ag-cn5.envisioniot.com","access_key":"AK1","secret_key":"SK1","org_id":"o123"},"replay":{"enabled":false,"mode":"replay","replay_start_time":"2026-09-01 13:46:00","fetch_interval_sec":30,"lookback_sec":300,"delay_sec":0,"target_lag_sec":0,"assets":[{"asset_id":"yARGkDuq","point_ids":["p1","p2"]}],"mappings":[{"source":"yARGkDuq/p1","target":"dev1.ActivePW"}]}}'
echo "== T3 GET roundtrip"; curl -s -m 5 -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-config
echo "== T4 files on disk"; D=$R/config/py_instances/$ID; [ -d $D ] || D=$R/config/py_simulator; echo "dir=$D"; ls -la $D/config | grep tsdb; cat $D/config/tsdb_replay_config.json
echo "== T5 tsdb-test valid"; curl -s -m 5 -X POST -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-test -H 'Content-Type: application/json' -d '{"apigw_address":"a","access_key":"b","secret_key":"c","org_id":"d"}'
echo "== T6 tsdb-test missing"; curl -s -m 5 -X POST -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-test -H 'Content-Type: application/json' -d '{"apigw_address":"a"}'
echo "== T7 bad id"; curl -s -m 5 -w "\nHTTP %{http_code}\n" $A/py-microgrid/nonexist/tsdb-config
echo "== T8 bad json"; curl -s -m 5 -X PUT -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-config -d '{bad'
echo "== T9 bad method"; curl -s -m 5 -X DELETE -w "\nHTTP %{http_code}\n" $A/py-microgrid/$ID/tsdb-config
echo "== R1 regression config/topology"; curl -s -m 5 -o /dev/null -w "config HTTP %{http_code}\n" $A/py-microgrid/$ID/config; curl -s -m 5 -o /dev/null -w "topology HTTP %{http_code}\n" $A/py-microgrid/$ID/topology
echo "== F1 frontend"; curl -s -m 5 -o /dev/null -w "index HTTP %{http_code}\n" http://127.0.0.1:18989/
grep -l "tsdb-config" $R/web/dist/assets/js/*.js | head
echo "== log"; tail -8 $R/logs/output.log
