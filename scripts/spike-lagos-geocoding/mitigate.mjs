// Scored classification of the 17 retrieved mentions, with OSM match category,
// so post-processing rules can be evaluated instead of guessed at.
const M=[
 {n:'Ijede',           cat:'place',   s:'exact'},
 {n:'Oko Ope',         cat:'none',    s:'failed'},
 {n:'Anjorin',         cat:'highway', s:'wrong'},   // Anjorin St, Alimosho ~32km from Ikorodu
 {n:'Abule Eko',       cat:'place',   s:'exact'},
 {n:'Odetedo',         cat:'none',    s:'failed'},
 {n:'Ikota',           cat:'place',   s:'exact'},
 {n:'Kusenla Road',    cat:'highway', s:'exact'},   // genuinely a road; query says "Road"
 {n:'MMIA',            cat:'none',    s:'failed'},
 {n:'Lekki (30 Jun)',  cat:'boundary',s:'wrong'},   // Ibeju-Lekki, ~39km E, wrong LGA
 {n:'Okota',           cat:'highway', s:'exact'},
 {n:'Oshodi',          cat:'place',   s:'exact'},
 {n:'Okokomaiko',      cat:'place',   s:'exact'},
 {n:'Ipaja',           cat:'highway', s:'close'},   // Ipaja Rd in Agege, ~3km, adjacent LGA
 {n:'Ikoyi',           cat:'place',   s:'exact'},
 {n:'Lekki (14 Jul)',  cat:'boundary',s:'wrong'},
 {n:'Victoria Island', cat:'place',   s:'exact'},
 {n:'Oworonshoki',     cat:'highway', s:'wrong'},   // Apapa-Oworonshoki Expwy ~11km, wrong LGA
]
const ROADWORD=/road|street|expressway|avenue|crescent/i
function tally(rows,label){
 const c={exact:0,close:0,wrong:0,failed:0}
 for(const r of rows) c[r.s]++
 const n=rows.length, eoc=c.exact+c.close
 console.log(`${label.padEnd(46)} exact ${c.exact}  close ${c.close}  wrong ${c.wrong}  failed ${c.failed}`)
 console.log(`${''.padEnd(46)} exact-or-close ${eoc}/${n} = ${(100*eoc/n).toFixed(1)}% (bar >80% → ${100*eoc/n>80?'pass':'**FAIL**'})   wrong ${c.wrong}/${n} = ${(100*c.wrong/n).toFixed(1)}% (bar <5% → ${100*c.wrong/n<5?'pass':'**FAIL**'})\n`)
}
tally(M,'0. As measured (no mitigation)')
tally(M.map(r=>r.cat==='highway'?{...r,s:'failed'}:r),'1. Blanket: reject every highway match')
tally(M.map(r=>(r.cat==='highway'&&!ROADWORD.test(r.n))?{...r,s:'failed'}:r),'2. Smart: reject highway unless query names a road')
tally(M.map(r=>(r.cat==='highway'&&!ROADWORD.test(r.n))||r.cat==='boundary'?{...r,s:'failed'}:r),'3. Smart + reject LGA-boundary centroids')
