const { Firestore } = require('@google-cloud/firestore');

// Connect to Firestore in the default GCP project
const db = new Firestore();

/**
 * Cloud Run Function HTTP Entry Point
 * Renders the zlist table without image handling or parameter z logic.
 */
exports.handleView = async (req, res) => {
  try {
    const snapshot = await db.collection('zlist').get();
    const now = Date.now();

    let tableRowsHtml = '';

    snapshot.docs.forEach((doc) => {
      const data = doc.data();
      const h = data.h !== undefined ? data.h : doc.id;
      const i = data.i !== undefined ? data.i : '';
      const r = data.r !== undefined ? data.r : '';
      const d = data.d !== undefined ? Number(data.d) : 0;
      const c = data.c !== undefined ? Number(data.c) : 0;

      // Calculate elapsed time (tt) in minutes from timestamp `t`
      let tt = 0;
      if (data.t) {
        const tDate = typeof data.t.toDate === 'function' ? data.t.toDate() : new Date(data.t);
        tt = Math.floor((now - tDate.getTime()) / (1000 * 60));
      }

      // Replicate original conditional background styling
      let rowStyle = '';
      if (tt > 200) {
        rowStyle = " style='background-color: lightgrey;'";
      } else if (tt > 60) {
        rowStyle = " style='background-color: pink;'";
      } else if (tt > 30) {
        rowStyle = " style='background-color: yellow;'";
      } else if (d > 5) {
        rowStyle = " style=' color: red;'";
      }

      tableRowsHtml += `\t<tr${rowStyle}>\n`;
      tableRowsHtml += `\t\t<td>${escapeHtml(h)}</td>\n`;
      tableRowsHtml += `\t\t<td>${escapeHtml(i)}</td>\n`;
      tableRowsHtml += `\t\t<td>${escapeHtml(r)}</td>\n`;
      tableRowsHtml += `\t\t<td>${d}</td>\n`;
      tableRowsHtml += `\t\t<td>${c}</td>\n`;
      tableRowsHtml += `\t\t<td>${tt}</td>\n`;
      tableRowsHtml += `\t</tr>\n`;
    });

    const htmlResponse = `<!doctype html>
<html lang="en">
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="https://code.jquery.com/mobile/1.4.5/jquery.mobile-1.4.5.min.css">
  <script src="https://code.jquery.com/jquery-1.11.3.min.js"></script>
  <script src="https://code.jquery.com/mobile/1.4.5/jquery.mobile-1.4.5.min.js"></script>
</head>
<body>
<table style="width: 100%;">
${tableRowsHtml}</table>
</body>
</html>`;

    res.status(200).set('Content-Type', 'text/html').send(htmlResponse);
  } catch (err) {
    console.error('Error fetching zlist:', err);
    res.status(500).send('Internal Server Error');
  }
};

function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}