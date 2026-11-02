const { Firestore, FieldValue } = require('@google-cloud/firestore');

// Initializes Firestore using Application Default Credentials (ADC) in GCP
const db = new Firestore();

/**
 * Cloud Run Function HTTP Entry Point
 */
exports.handleRequest = async (req, res) => {
  const h = (req.query.h || '').toString();
  const c = req.query.c !== undefined ? req.query.c.toString() : '';
  const r = req.query.r !== undefined ? req.query.r.toString() : '-';

  // Extract client IP address
  let remoteAddr = req.headers['x-forwarded-for'] || req.socket.remoteAddress || '';
  if (remoteAddr.includes(',')) {
    remoteAddr = remoteAddr.split(',')[0].trim();
  }

  // Perform custom IP transformation (matching PHP logic: octet 0.1.(octet3+1).2)
  let i = remoteAddr;
  const cleanIp = i.replace(/^.*:/, ''); // Strip IPv6 prefix if present (e.g., ::ffff:)
  const ia = cleanIp.split('.');
  if (ia.length === 4) {
    const p3 = parseInt(ia[3], 10) + 1;
    i = `${ia[0]}.${ia[1]}.${p3}.${ia[2]}`;
  }

  // Validate parameter 'h': 1 to 3 non-digits followed by 3 digits
  if (!/\D{1,3}\d{3}/.test(h)) {
    return res.status(200).send('0');
  }

  try {
    if (/r/.test(r)) {
      // PHP: UPDATE zlist SET r=0;
      const snapshot = await db.collection('zlist').get();
      if (!snapshot.empty) {
        const batch = db.batch();
        snapshot.docs.forEach((doc) => {
          batch.update(doc.ref, { r: 0 });
        });
        await batch.commit();
      }
    } else if (/\d+/.test(r)) {
      // PHP: UPDATE zlist SET r="$r" WHERE h="$h";
      await db.collection('zlist').doc(h).set({ r: r }, { merge: true });
    } else if (/c/.test(c)) {
      // PHP: INSERT INTO zlist (h,i,c,t) VALUES ("$h","$i",0,CURRENT_TIMESTAMP) ON DUPLICATE KEY UPDATE i="$i", t=CURRENT_TIMESTAMP;
      const docRef = db.collection('zlist').doc(h);
      const docSnap = await docRef.get();
      if (!docSnap.exists) {
        await docRef.set({
          h: h,
          i: i,
          c: 0,
          t: FieldValue.serverTimestamp()
        });
      } else {
        await docRef.update({
          i: i,
          t: FieldValue.serverTimestamp()
        });
      }
    } else if (/0/.test(c)) {
      const batch = db.batch();

      // 1. PHP: UPDATE zlist SET c=0 WHERE h LIKE CONCAT('%',SUBSTRING("$h",-3,3));
      const suffix = h.slice(-3);
      const snapshot = await db.collection('zlist').get();
      snapshot.docs.forEach((doc) => {
        if (doc.id.endsWith(suffix) || (doc.data().h && doc.data().h.endsWith(suffix))) {
          batch.update(doc.ref, { c: 0 });
        }
      });

      // 2. PHP: DELETE FROM zlist WHERE t < DATE_SUB(NOW(), INTERVAL 1 DAY);
      const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
      const oldZlist = await db.collection('zlist').where('t', '<', oneDayAgo).get();
      oldZlist.docs.forEach((doc) => {
        batch.delete(doc.ref);
      });

      // 3. PHP: DELETE FROM zda WHERE ct < DATE_SUB(NOW(), INTERVAL 15 DAY);
      const fifteenDaysAgo = new Date(Date.now() - 15 * 24 * 60 * 60 * 1000);
      const oldZda = await db.collection('zda').where('ct', '<', fifteenDaysAgo).get();
      oldZda.docs.forEach((doc) => {
        batch.delete(doc.ref);
      });

      await batch.commit();
    } else if (/-/.test(r)) {
      // PHP:
      // UPDATE zlist SET d=0 WHERE h="$h" AND i<>"$i";
      // UPDATE zlist SET d=d+1 WHERE h="$h" AND i="$i";
      // INSERT INTO zlist (h,i,c,t) VALUES ("$h","$i",1,CURRENT_TIMESTAMP) ON DUPLICATE KEY UPDATE c=c+1, i="$i", t=CURRENT_TIMESTAMP;
      const docRef = db.collection('zlist').doc(h);

      await db.runTransaction(async (transaction) => {
        const docSnap = await transaction.get(docRef);
        if (!docSnap.exists) {
          transaction.set(docRef, {
            h: h,
            i: i,
            c: 1,
            d: 0,
            t: FieldValue.serverTimestamp()
          });
        } else {
          const data = docSnap.data();
          const currentD = data.d || 0;
          const currentC = data.c || 0;
          const newD = data.i === i ? currentD + 1 : 0;

          transaction.update(docRef, {
            d: newD,
            c: currentC + 1,
            i: i,
            t: FieldValue.serverTimestamp()
          });
        }
      });
    }

    return res.status(200).send(i);
  } catch (err) {
    console.error('Error executing Firestore operations:', err);
    return res.status(500).send('Internal Server Error');
  }
};