let timer = null;

self.onmessage = function (e) {
  if (e.data === 'start') {
    if (timer) clearInterval(timer);
    // Pulse every 5 seconds: Web Worker timers are immune to browser tab throttling / sleeping
    timer = setInterval(function () {
      self.postMessage('ping');
    }, 5000);
  } else if (e.data === 'stop') {
    if (timer) clearInterval(timer);
    timer = null;
  }
};
