<!DOCTYPE html>
<html>
<head>
<title>Echelon Resistance vs Encoder Position</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.4/dist/chart.umd.min.js"></script>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #1a1a2e; color: #e0e0e0; margin: 0; padding: 20px; }
  .container { max-width: 1100px; margin: 0 auto; }
  h1 { color: #fff; font-size: 1.4em; margin-bottom: 4px; }
  .subtitle { color: #888; font-size: 0.85em; margin-bottom: 20px; }
  .chart-container { background: #16213e; border-radius: 12px; padding: 20px; margin-bottom: 20px; }
  canvas { width: 100% !important; }
  .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 20px; }
  .stat { background: #16213e; border-radius: 8px; padding: 14px; }
  .stat-value { font-size: 1.6em; font-weight: bold; color: #4cc9f0; }
  .stat-label { font-size: 0.8em; color: #888; margin-top: 4px; }
</style>
</head>
<body>
<div class="container">
  <h1>B03 — Resistance Level vs Cumulative Encoder Movement</h1>
  <p class="subtitle">Echelon EX-3 • 32-level sweep (1→32→1) • 2026-10-04</p>
  
  <div class="stats">
    <div class="stat"><div class="stat-value">32</div><div class="stat-label">Resistance levels</div></div>
    <div class="stat"><div class="stat-value">44</div><div class="stat-label">Peak encoder steps (ADC)</div></div>
    <div class="stat"><div class="stat-value">~1.4×</div><div class="stat-label">Encoder steps per level</div></div>
    <div class="stat"><div class="stat-value">+2</div><div class="stat-label">Net encoder drift (return)</div></div>
  </div>

  <div class="chart-container">
    <canvas id="timeChart" height="110"></canvas>
  </div>
  
  <div class="chart-container">
    <canvas id="scatterChart" height="100"></canvas>
  </div>
</div>

<script>
const data = {"times": [0.0, 1.8, 3.7, 5.5, 7.3, 9.2, 11.0, 12.9, 14.7, 16.5, 18.4, 20.2, 22.0, 23.9, 25.7, 27.5, 29.4, 31.2, 33.0, 34.9, 36.7, 38.6, 40.4, 42.2, 44.1, 45.9, 47.7, 49.6, 51.4, 53.2, 55.1, 56.9, 58.8, 60.6, 62.4, 64.3, 66.1, 67.9, 69.8, 71.6, 73.4, 75.3, 77.1, 78.9, 80.8, 82.6, 84.5, 86.3, 88.1, 90.0, 91.8, 93.6, 95.5, 97.3, 99.1, 101.0, 102.8, 104.7, 106.5, 108.3, 110.2, 112.0, 113.8, 115.7, 117.5, 119.3, 121.2, 123.0, 124.8, 126.7, 128.5, 130.4, 132.2, 134.0, 135.9, 137.7, 139.5, 141.4, 143.2, 145.0, 146.9, 148.7, 150.5, 152.4, 154.2, 156.1, 157.9, 159.7, 161.6, 163.4, 165.2, 167.1, 168.9, 170.7, 172.6, 174.4, 176.3, 178.1, 179.9, 181.8, 183.6, 185.4, 187.3, 189.1, 190.9, 192.8, 194.6, 196.4, 198.3, 200.1, 202.0, 203.8, 205.6, 207.5, 209.3, 211.1, 213.0, 214.8, 216.6, 218.5, 220.3, 222.2, 224.0, 225.8, 227.7, 229.5, 231.3, 233.2, 235.0, 236.8, 238.7, 240.5, 242.3, 244.2, 246.0, 247.9, 249.7, 251.5, 253.4, 255.2, 257.0, 258.9, 260.7, 262.5, 264.4, 266.2, 268.1, 269.9, 271.7, 273.6, 275.4, 277.2, 279.1, 280.9, 282.7, 284.6, 286.4, 288.2, 290.1, 291.9, 293.8, 295.6, 297.4, 299.3, 301.1, 302.9, 304.8, 306.6, 308.4, 310.3, 312.1, 313.9, 315.8, 317.6, 319.5, 321.3, 323.1, 325.0, 326.8, 328.6, 330.5, 332.3, 334.1, 336.0, 337.8, 339.7, 341.5, 343.3, 345.2, 347.0, 348.8, 350.7, 352.5, 354.3, 356.2, 358.0, 359.9, 361.7, 363.5, 365.4, 367.2, 369.0], "wipers": [162, 163, 162, 162, 162, 162, 163, 162, 162, 184, 184, 184, 225, 224, 224, 239, 259, 260, 259, 298, 298, 298, 274, 297, 297, 340, 340, 340, 274, 374, 374, 374, 356, 408, 409, 409, 412, 443, 443, 443, 384, 481, 481, 465, 525, 550, 531, 560, 561, 560, 539, 598, 598, 598, 542, 632, 632, 632, 633, 669, 669, 639, 713, 713, 713, 713, 646, 747, 747, 754, 768, 785, 785, 772, 820, 795, 858, 857, 795, 895, 895, 971, 971, 850, 938, 938, 937, 942, 967, 973, 1007, 998, 1045, 1045, 1017, 1085, 1085, 1045, 1123, 1121, 1077, 1161, 1161, 1161, 1136, 1198, 1198, 1183, 1239, 1239, 1239, 1178, 1274, 1274, 1274, 1273, 1331, 1331, 1331, 1331, 1211, 1274, 1274, 1194, 1238, 1238, 1163, 1197, 1159, 1125, 1161, 1162, 1084, 1125, 1079, 1055, 1084, 949, 1045, 999, 987, 1007, 910, 969, 969, 899, 931, 933, 850, 893, 851, 826, 815, 784, 779, 733, 781, 781, 680, 784, 797, 648, 744, 610, 710, 611, 670, 670, 554, 632, 659, 493, 599, 542, 526, 560, 420, 525, 494, 419, 482, 351, 443, 357, 409, 351, 340, 375, 347, 335, 308, 272, 272, 236, 235, 200, 162, 161, 163, 162, 161, 163], "positions": [0, 0, 0, 0, 0, 0, 0, 2, 2, 4, 4, 6, 8, 8, 10, 10, 10, 10, 15, 16, 16, 17, 19, 26, 26, 27, 27, 27, 26, 26, 26, 28, 28, 28, 28, 29, 29, 29, 29, 29, 27, 27, 27, 32, 32, 34, 34, 34, 34, 34, 35, 35, 35, 35, 36, 36, 36, 36, 36, 36, 36, 38, 38, 38, 38, 38, 38, 38, 38, 38, 38, 38, 38, 38, 38, 37, 37, 37, 38, 38, 40, 40, 38, 38, 38, 38, 38, 38, 38, 38, 38, 39, 39, 39, 40, 40, 40, 40, 40, 40, 40, 40, 41, 41, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 44, 44, 44, 44, 44, 42, 42, 42, 40, 37, 37, 36, 36, 36, 34, 34, 33, 33, 32, 32, 32, 32, 31, 29, 27, 27, 28, 26, 26, 26, 26, 26, 26, 26, 21, 21, 21, 21, 20, 20, 20, 20, 20, 20, 18, 18, 18, 18, 16, 16, 16, 13, 13, 13, 13, 13, 12, 12, 12, 10, 10, 9, 9, 9, 10, 10, 10, 10, 10, 10, 10, 9, 9, 9, 10, 9, 8, 8, 8, 5, 4, 4, 4, 2, 2, 2, 2, 2]};

// Time series chart
new Chart(document.getElementById('timeChart'), {
  type: 'line',
  data: {
    labels: data.times,
    datasets: [
      {
        label: 'Pot Wiper (ADC)',
        data: data.wipers,
        borderColor: '#4cc9f0',
        backgroundColor: 'rgba(76,201,240,0.1)',
        fill: true,
        borderWidth: 1.5,
        pointRadius: 0,
        yAxisID: 'y'
      },
      {
        label: 'Cumulative Encoder Steps',
        data: data.positions,
        borderColor: '#f72585',
        backgroundColor: 'rgba(247,37,133,0.1)',
        fill: true,
        borderWidth: 2,
        pointRadius: 0,
        yAxisID: 'y1'
      }
    ]
  },
  options: {
    responsive: true,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      title: { display: true, text: 'Resistance & Encoder vs Time', color: '#ccc', font: { size: 14 } },
      legend: { labels: { color: '#aaa' } }
    },
    scales: {
      x: { title: { display: true, text: 'Time (s)', color: '#888' }, ticks: { color: '#666', maxTicksLimit: 20 }, grid: { color: '#333' } },
      y: { position: 'left', title: { display: true, text: 'Pot Wiper (ADC)', color: '#4cc9f0' }, ticks: { color: '#4cc9f0' }, grid: { color: '#222' } },
      y1: { position: 'right', title: { display: true, text: 'Encoder Steps', color: '#f72585' }, ticks: { color: '#f72585' }, grid: { drawOnChartArea: false } }
    }
  }
});

// Scatter: Encoder Position vs Wiper
const scatterData = data.wipers.map((w, i) => ({ x: data.positions[i], y: w }));
new Chart(document.getElementById('scatterChart'), {
  type: 'scatter',
  data: {
    datasets: [{
      label: 'Wiper vs Encoder Position',
      data: scatterData,
      backgroundColor: (ctx) => {
        const idx = ctx.dataIndex;
        // Color by time: ascending = cyan, descending = pink
        return idx < data.times.length / 2 ? 'rgba(76,201,240,0.5)' : 'rgba(247,37,133,0.5)';
      },
      pointRadius: 3
    }]
  },
  options: {
    responsive: true,
    plugins: {
      title: { display: true, text: 'Pot Wiper vs Cumulative Encoder Steps (cyan=ascending, pink=descending)', color: '#ccc', font: { size: 14 } },
      legend: { display: false }
    },
    scales: {
      x: { title: { display: true, text: 'Cumulative Encoder Steps', color: '#888' }, ticks: { color: '#666' }, grid: { color: '#333' } },
      y: { title: { display: true, text: 'Pot Wiper (ADC)', color: '#888' }, ticks: { color: '#666' }, grid: { color: '#222' } }
    }
  }
});
</script>
</body>
</html>
