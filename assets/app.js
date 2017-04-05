window.onload = function () {
    var conn;
    var msg = document.getElementById("msg");
    var log = document.getElementById("log");
    function appendLog(item) {
        var doScroll = log.scrollTop > log.scrollHeight - log.clientHeight - 1;
        log.appendChild(item);
        if (doScroll) {
            log.scrollTop = log.scrollHeight - log.clientHeight;
        }
    }
    document.getElementById("form").onsubmit = function () {
        if (!conn) {
            return false;
        }
        if (!msg.value) {
            return false;
        }
        conn.send(msg.value);
        msg.value = "";
        return false;
    };
    if (window["WebSocket"]) {
        var now = new Date()
        var t = now.getTime()
        var chart = c3.generate({
          bindto: '#chart',
          data: {
            x: 'Time',
            columns: [
              ['Time', t],
              ['Connection', 1]
            ]
          },
          axis : {
              x : {
                  type : 'timeseries',
                  tick: {
                      format: '%H:%M:%S'
                  }
              }
          }
        });
        conn = new WebSocket("ws://" + document.location.host + "/status");
        stats = [];
        times = [];
        conn.onopen = function (evt) {
          var item = document.createElement("div");
          item.innerHTML = "<b>Connection opened.</b>";
          appendLog(item);
          conn.send("init")
        };
        conn.onclose = function (evt) {
            var item = document.createElement("div");
            item.innerHTML = "<b>Connection closed.</b>";
            appendLog(item);
        };
        conn.onmessage = function (evt) {
            var item = document.createElement("div");
            var message = evt.data;
            if (message.charAt(0) == '[') {
              message = JSON.parse(message)
              times = message[0].map(function(x){
                return Number(x)
              })
              statuses = message[1].map(function(y){
                if (y == 'true'){
                  return 1
                }
                if (y == 'false'){
                  return 0
                }
              })
            } else {
              var splitMsg = message.split(" ")
              var stamp = Number(splitMsg[1])
              splitMsg[1] = Date(stamp).toString()
              message = splitMsg.join(" ")
              item.innerText = message
              var d = message.includes("down")
              var u = message.includes("up")

              times.push(stamp)
              if (d == true) {
                stats.push(0)
              };
              if (u == true) {
                stats.push(1)
              };
              appendLog(item);
            }
            chart.load({
              columns: [
                ['Time', t].concat(times),
                ['Connection', 1].concat(stats)
              ]
            });
        };
    } else {
        var item = document.createElement("div");
        item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
        appendLog(item);
    };
};
