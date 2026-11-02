const { Firestore } = require('@google-cloud/firestore');

// Connect to Firestore in the GCP project using Application Default Credentials
const db = new Firestore();

/**
 * Cloud Run Function HTTP Entry Point for h.php replacement
 */
exports.handleH = async (req, res) => {
  const h = (req.query.h || '').toString();

  try {
    if (/\D{1,3}\d{3}/.test(h)) {
      // 1. PHP: SELECT i FROM zlist WHERE h="$h"
      const docRef = db.collection('zlist').doc(h);
      const docSnap = await docRef.get();

      let resultStr = '';
      if (docSnap.exists) {
        const data = docSnap.data();
        if (data.i) {
          const ia = data.i.split('.');
          if (ia.length === 4) {
            // PHP: $ib = $ia[0].".".$ia[1].".".($ia[3]).".".($ia[2]-1);
            const p2 = parseInt(ia[2], 10) - 1;
            resultStr = `${ia[0]}.${ia[1]}.${ia[3]}.${p2}`;
          }
        }
      }

      return res.status(200).send(resultStr);

    } else if (/t30/.test(h)) {
      // 2. PHP: SELECT h FROM zlist WHERE TIMESTAMPDIFF(MINUTE,t,NOW()) > 30
      const thirtyMinutesAgo = new Date(Date.now() - 30 * 60 * 1000);
      const snapshot = await db.collection('zlist')
        .where('t', '<', thirtyMinutesAgo)
        .get();

      let output = '';
      snapshot.docs.forEach((doc) => {
        const hVal = doc.data().h || doc.id;
        output += `${hVal},`;
      });

      return res.status(200).send(output);

    } else {
      // 3. Fallback
      return res.status(200).send('0');
    }
  } catch (err) {
    console.error('Error executing query:', err);
    return res.status(500).send('Internal Server Error');
  }
};