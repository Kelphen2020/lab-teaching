import { build } from 'vite'
build().then(() => { console.log('BUILD_OK'); process.exit(0) }).catch(e => { console.error('BUILD_FAIL', e.message); process.exit(1) })
