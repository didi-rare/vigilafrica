// Clopper-Pearson exact binomial CI via incomplete beta (continued fraction).
function betacf(a,b,x){const M=200,e=3e-12;let qab=a+b,qap=a+1,qam=a-1,c=1,d=1-qab*x/qap
 if(Math.abs(d)<1e-30)d=1e-30;d=1/d;let h=d
 for(let m=1;m<=M;m++){const m2=2*m
  let aa=m*(b-m)*x/((qam+m2)*(a+m2));d=1+aa*d;if(Math.abs(d)<1e-30)d=1e-30
  c=1+aa/c;if(Math.abs(c)<1e-30)c=1e-30;d=1/d;h*=d*c
  aa=-(a+m)*(qab+m)*x/((a+m2)*(qap+m2));d=1+aa*d;if(Math.abs(d)<1e-30)d=1e-30
  c=1+aa/c;if(Math.abs(c)<1e-30)c=1e-30;d=1/d;const del=d*c;h*=del
  if(Math.abs(del-1)<e)break}
 return h}
function lgamma(z){const g=[676.5203681218851,-1259.1392167224028,771.32342877765313,-176.61502916214059,12.507343278686905,-0.13857109526572012,9.9843695780195716e-6,1.5056327351493116e-7]
 if(z<0.5)return Math.log(Math.PI/Math.sin(Math.PI*z))-lgamma(1-z)
 z-=1;let x=0.99999999999980993;for(let i=0;i<8;i++)x+=g[i]/(z+i+1)
 const t=z+7.5;return 0.5*Math.log(2*Math.PI)+(z+0.5)*Math.log(t)-t+Math.log(x)}
function ibeta(a,b,x){if(x<=0)return 0;if(x>=1)return 1
 const f=Math.exp(lgamma(a+b)-lgamma(a)-lgamma(b)+a*Math.log(x)+b*Math.log(1-x))
 return x<(a+1)/(a+b+2)?f*betacf(a,b,x)/a:1-f*betacf(b,a,1-x)/b}
function betaInv(p,a,b){let lo=0,hi=1;for(let i=0;i<200;i++){const m=(lo+hi)/2;ibeta(a,b,m)<p?lo=m:hi=m}return (lo+hi)/2}
function cp(k,n){const a=0.05
 const lo=k===0?0:betaInv(a/2,k,n-k+1)
 const hi=k===n?1:betaInv(1-a/2,k+1,n-k)
 return [lo*100,hi*100]}
const pct=(k,n)=>(100*k/n).toFixed(1)
function row(label,k,n,bar,dir){const[l,h]=cp(k,n)
 const p=100*k/n
 const pass=dir==='>'?p>bar:p<bar
 console.log(`${label.padEnd(30)} ${String(k)+'/'+n} = ${pct(k,n).padStart(5)}%   95% CI ${l.toFixed(1)}–${h.toFixed(1)}%   bar ${dir}${bar}%  → ${pass?'PASS':'**FAIL**'}`)}

console.log('=== STRICT scoring (n=17 retrieved locality mentions) ===')
row('Exact',9,17,null,'>');
row('Close',1,17,null,'>');
row('Wrong',4,17,5,'<');
row('Failed',3,17,null,'>');
console.log('')
row('Exact-or-close (bar >80%)',10,17,80,'>')
row('Wrong (bar <5%)',4,17,5,'<')
console.log('\n=== SENSITIVITY: treat both "Lekki" as Close, not Wrong ===')
row('Exact-or-close',12,17,80,'>')
row('Wrong',2,17,5,'<')
console.log('\n=== SENSITIVITY: most generous possible — all 4 Wrong reclassed Close ===')
row('Exact-or-close',14,17,80,'>')
row('Wrong',0,17,5,'<')
