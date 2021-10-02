// THIS SUCKS.

if (!Uint8Array.prototype.slice) {
  Object.defineProperty(Uint8Array.prototype, 'slice', {
    value: function(begin, end) {
      return new Uint8Array(Array.prototype.slice.call(this, begin, end));
    }
  });
}

var verbose = true;
var streamingStarted = false;
var ms = new MediaSource();
var queue = [];
var ws;

function pushPacket(data) {
  if (!streamingStarted) {
    sourceBuffer.appendBuffer(data);
    streamingStarted = true;
    return;
  }
  queue.push(data);
  if (!sourceBuffer.updating) {
    loadPacket();
  }
}

function loadPacket() {
  if (!sourceBuffer.updating) {
    if (queue.length > 0) {
      sourceBuffer.appendBuffer(queue.shift());
    } else {
      streamingStarted = false;
    }
  }
}

var potocol = 'ws';
if (location.protocol.indexOf('s') >= 0) {
  potocol = 'wss';
}

function opened() {
  var inputVal = $('#suuid').val();
  var port = $('#port').val();
  ws = new WebSocket(potocol + "://127.0.0.1"+port+"/ws/live?suuid="+inputVal);
  ws.binaryType = "arraybuffer";
  ws.onopen = function(event) {
    console.log('Connect');
  }
  ws.onmessage = function(event) {
    var data = new Uint8Array(event.data);
    if (data[0] == 9) {
      decoded_arr=data.slice(1);
      if (window.TextDecoder) {
        mimeCodec = new TextDecoder("utf-8").decode(decoded_arr);
      } else {
        mimeCodec = Utf8ArrayToStr(decoded_arr);
      }
      if (verbose) {
        console.log('first packet with codec data: ' + mimeCodec);
      }
      sourceBuffer = ms.addSourceBuffer('video/mp4; codecs="' + mimeCodec + '"');
      sourceBuffer.mode = "segments"
      sourceBuffer.addEventListener("updateend", loadPacket);
    } else {
      pushPacket(event.data);
    }
  };
}

function Utf8ArrayToStr(array) {
  var out, i, len, c;
  var char2, char3;
  out = "";
  len = array.length;
  i = 0;
  while (i < len) {
    c = array[i++];
    switch (c >> 4) {
      case 7:
        out += String.fromCharCode(c);
        break;
      case 13:
        char2 = array[i++];
        out += String.fromCharCode(((c & 0x1F) << 6) | (char2 & 0x3F));
        break;
      case 14:
        char2 = array[i++];
        char3 = array[i++];
        out += String.fromCharCode(((c & 0x0F) << 12) |
          ((char2 & 0x3F) << 6) |
          ((char3 & 0x3F) << 0));
        break;
    }
  }
  return out;
}

var livestream = document.getElementById('livestream');

function startup() {
  ms.addEventListener('sourceopen', opened, false);
  livestream.src = window.URL.createObjectURL(ms);
}

$(document).ready(function() {
startup();
  var suuid = $('#suuid').val();
  $('#'+suuid).addClass('active');
});
