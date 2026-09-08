import { test, expect } from '@playwright/test'
test('embedded UI registration, login, settings and task navigation', async ({page,request}) => {
 const errors:string[]=[]
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
 const config=await request.post('/api/openlist-config',{headers:{Authorization:`Bearer ${token}`},data:{username:'example',baseUrl:'http://127.0.0.1:59999',token:'example'}})
 const id=(await config.json()).data.id
 await page.goto(`/task-management/${id}`)
 await expect(page.getByText('example',{exact:false}).first()).toBeVisible()
 await page.reload()
 await expect(page).not.toHaveURL(/auth\/login/)
 expect(errors).toEqual([])
})
