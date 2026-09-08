import { createServer } from 'node:http'
import { once } from 'node:events'
import { test, expect } from '@playwright/test'
test('embedded UI registration, login, settings and task navigation', async ({page,request}) => {
 const server=createServer((req,res)=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({code:200,data:{content:[{name:'Mayday.2026.1080p.mkv',is_dir:false,size:10,sign:'test'},{name:'Mayday.2026.2160p.DV.mkv',is_dir:false,size:20,sign:'test'}],total:2}}))})
 server.listen(0,'127.0.0.1');await once(server,'listening')
 const port=(server.address() as {port:number}).port
 const errors:string[]=[]
 page.on('dialog',d=>d.accept())
 page.on('pageerror', e=>errors.push(e.message))
 const registered=await request.post('/api/auth/sign-up',{data:{username:'browser',password:'browser-secret'}})
 expect((await registered.json()).code).toBe(200)
 await page.goto('/auth/login')
 await page.locator('input[name=username]').fill('browser')
 await page.locator('input[name=password]').fill('browser-secret')
 await page.locator('button[type=submit]').first().click()
 await expect(page).not.toHaveURL(/auth\/login/)
 await page.goto('/settings')
 await expect(page.locator('#tmdbLanguage')).toBeVisible()
 const login=await request.post('/api/auth/sign-in',{data:{username:'browser',password:'browser-secret'}})
 const token=(await login.json()).data.token
 const config=await request.post('/api/openlist-config',{headers:{Authorization:`Bearer ${token}`},data:{username:'example',baseUrl:`http://127.0.0.1:${port}`,token:'example'}})
 const id=(await config.json()).data.id
 const task=await request.post('/api/task-config',{headers:{Authorization:`Bearer ${token}`},data:{taskName:'browser task',openlistConfigId:id,path:'/media',libraryType:'movie',isActive:true}})
 expect((await task.json()).code).toBe(200)
 await page.goto(`/task-management/${id}`)
 await expect(page.getByText('example',{exact:false}).first()).toBeVisible()
 await page.reload()
 await expect(page).not.toHaveURL(/auth\/login/)
 await page.getByText('画质筛选预览',{exact:true}).click()
 await expect(page.getByText('共 2 个视频，保留 1 个，过滤 1 个。',{exact:true})).toBeVisible()
 await expect(page.getByText('保留：/media/Mayday.2026.2160p.DV.mkv',{exact:true})).toBeVisible()
 await expect(page.getByText('过滤：/media/Mayday.2026.1080p.mkv',{exact:true})).toBeVisible()
 await page.getByRole('button',{name:'关闭',exact:true}).click()
 await page.getByTitle('立即执行' ,{exact:true}).click()
 await page.getByText('全量执行',{exact:true}).click()
 await expect(page.getByText('已完成 · FINALIZE',{exact:true})).toBeVisible({timeout:15000})
 await page.getByTitle('手动刮削',{exact:true}).click()
 await expect(page).toHaveURL(/manual-scraping/)
 await expect(page.getByText('Mayday.2026.1080p.mkv',{exact:true}).first()).toBeVisible()
 server.close()
 expect(errors).toEqual([])
})
