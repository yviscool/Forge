import AxeBuilder from '@axe-core/playwright'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

const ADMIN_PASSWORD = 'E2E-Admin-Password-2026!'
const STUDENT_PASSWORD = 'E2E-Student-Password-2026!'

interface SeedResult {
  contestId: string
  contestName: string
  problemId: string
  username: string
}

async function adminLogin(request: APIRequestContext) {
  const response = await request.post('/api/v1/auth/login', {
    data: { login: 'admin', password: ADMIN_PASSWORD },
  })
  expect(response.status(), await response.text()).toBe(200)
}

function uniqueSuffix() {
  return `${Date.now()}${Math.random().toString(36).slice(2, 8)}`
}

async function createContestWithChoiceProblem(
  request: APIRequestContext,
  options: { student: boolean; problemType?: 'choice' | 'coding' }
): Promise<SeedResult> {
  await adminLogin(request)
  const suffix = uniqueSuffix()
  const contestName = `E2E Browser Contest ${suffix}`

  let username = ''
  if (options.student) {
    username = `e2e_${suffix}`
    const userResponse = await request.post('/api/v1/users', {
      data: {
        name: `E2E 浏览器选手 ${suffix}`,
        username,
        role: 'student',
        password: STUDENT_PASSWORD,
      },
    })
    expect(userResponse.status(), await userResponse.text()).toBe(201)
    const user = (await userResponse.json()) as { id: string }

    const contestResponse = await request.post('/api/v1/contests', {
      data: {
        name: contestName,
        description: 'Browser acceptance fixture',
        rankingMode: 'oi',
        allowedLanguages: ['cpp', 'python', 'go'],
      },
    })
    expect(contestResponse.status(), await contestResponse.text()).toBe(201)
    const contest = (await contestResponse.json()) as { id: string }

    const participantResponse = await request.post(
      `/api/v1/contests/${contest.id}/participants`,
      { data: { userId: user.id } }
    )
    expect(participantResponse.status(), await participantResponse.text()).toBe(200)

    const coding = options.problemType === 'coding'
    const problemResponse = await request.post(
      `/api/v1/contests/${contest.id}/problems`,
      {
        data: coding
          ? {
              code: 'E2E-CODE',
              title: 'E2E Coding Paper',
              statement: 'Write a program that prints the acceptance marker.',
              constraints: 'No additional constraints.',
              input: 'No input.',
              output: 'Print the marker.',
              taskType: 'traditional',
              timeLimitMs: 1000,
              memoryLimitMib: 256,
            }
          : {
              code: 'E2E-CHOICE',
              title: 'E2E Choice Paper',
              statement: 'Select the only valid option, then submit the answer sheet.',
              constraints: 'One answer.',
              input: '',
              output: '',
              taskType: 'choice',
              choiceQuestions: [
                {
                  id: 1,
                  type: 'single',
                  stem: 'Which option is the acceptance marker?',
                  options: ['A. alpha-marker', 'B. beta-marker'],
                  answer: 'A',
                  score: 10,
                  explanation: 'alpha-marker is the configured answer.',
                },
              ],
            },
      }
    )
    expect(problemResponse.status(), await problemResponse.text()).toBe(201)
    const problem = (await problemResponse.json()) as { id: string }

    const startResponse = await request.post(`/api/v1/contests/${contest.id}/start`)
    expect(startResponse.status(), await startResponse.text()).toBe(200)
    return { contestId: contest.id, contestName, problemId: problem.id, username }
  }

  const contestResponse = await request.post('/api/v1/contests', {
    data: {
      name: contestName,
      description: 'Teacher browser acceptance fixture',
      rankingMode: 'oi',
    },
  })
  expect(contestResponse.status(), await contestResponse.text()).toBe(201)
  const contest = (await contestResponse.json()) as { id: string }
  return { contestId: contest.id, contestName, problemId: '', username }
}

async function uiLogin(page: Page, username: string, password: string) {
  await page.goto('/app/login')
  await page.locator('#login-username').fill(username)
  await page.locator('#login-password').fill(password)
  await page.locator('form button[type="submit"]').click()
  await expect(page).toHaveURL(/\/app\/?$/)
}

async function expectNoHorizontalOverflow(page: Page) {
  const metrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }))
  expect(
    metrics.scrollWidth,
    `page overflows horizontally: ${metrics.scrollWidth}px > ${metrics.clientWidth}px`
  ).toBeLessThanOrEqual(metrics.clientWidth + 1)
}

async function expectNoSeriousAccessibilityViolations(page: Page) {
  // 扫描期间启用 reduced-motion：产品 CSS 会据此冻结入场动画，
  // axe 不会读到 480ms 淡入的中间态半透明文字而误报对比度。
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.evaluate(async () => {
    await document.fonts?.ready
    const animations = document.getAnimations().filter((animation) => {
      const timing = animation.effect?.getComputedTiming()
      return timing != null && Number.isFinite(timing.endTime)
    })
    await Promise.race([
      Promise.all(animations.map((animation) => animation.finished.catch(() => undefined))),
      new Promise<void>((resolve) => window.setTimeout(resolve, 1_000)),
    ])
  })
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()
  const blocking = results.violations.filter(
    (violation) => violation.impact === 'serious' || violation.impact === 'critical'
  )
  expect(
    blocking,
    blocking
      .map(
        (violation) =>
          `${violation.impact}: ${violation.id} - ${violation.help}\n${violation.nodes
            .map((node) => `  ${node.target.join(' ')}: ${node.failureSummary ?? ''}`)
            .join('\n')}`
      )
      .join('\n\n')
  ).toEqual([])
}

function collectPageFailures(page: Page) {
  const failures: string[] = []
  page.on('pageerror', (error) => failures.push(error.message))
  page.on('response', (response) => {
    if (response.status() >= 500) {
      failures.push(`${response.status()} ${response.request().method()} ${response.url()}`)
    }
  })
  page.on('requestfailed', (request) => {
    if (!request.url().includes('/api/v1/events')) {
      failures.push(
        `${request.method()} ${request.url()}: ${request.failure()?.errorText ?? 'request failed'}`
      )
    }
  })
  return failures
}

test('student can sign in, take a choice exam, recover the session, and sign out', async ({
  browserName,
  page,
  request,
}) => {
  const browserErrors = collectPageFailures(page)

  const fixture = await createContestWithChoiceProblem(request, { student: true })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/login')
  await expectNoHorizontalOverflow(page)
  await expectNoSeriousAccessibilityViolations(page)
  await page.locator('#login-username').fill(fixture.username)
  await page.locator('#login-password').fill(STUDENT_PASSWORD)
  const loginResponsePromise = page.waitForResponse(
    (response) =>
      response.url().endsWith('/api/v1/auth/login') &&
      response.request().method() === 'POST'
  )
  await page.locator('form button[type="submit"]').click()
  const loginResponse = await loginResponsePromise
  expect(loginResponse.status()).toBe(200)
  await expect(page).toHaveURL(/\/app\/?$/)
  await expectNoHorizontalOverflow(page)
  await expectNoSeriousAccessibilityViolations(page)

  // Set-Cookie is intentionally hidden from Chromium's response API. The Go
  // transport tests assert the authoritative header flags; here we verify the
  // browser actually stored a non-readable session cookie.
  const cookies = await page.context().cookies()
  const sessionCookie = cookies.find((cookie) => cookie.name === 'forge_session')
  expect(sessionCookie).toMatchObject({ httpOnly: true })
  // Playwright's WebKit cookie jar reports SameSite=None for Strict cookies
  // served over the plain-HTTP test origin. Production is HTTPS-only.
  if (browserName !== 'webkit') {
    expect(sessionCookie).toMatchObject({ sameSite: 'Strict' })
  }
  expect(
    await page.evaluate(() => localStorage.getItem('forge_session'))
  ).toBeNull()

  const card = page.locator(
    `[data-testid="contest-card"][data-contest-id="${fixture.contestId}"]`
  )
  await expect(card).toContainText(fixture.contestName)
  await card.locator('[data-testid="enter-contest"]').click()
  await expect(page).toHaveURL(new RegExp(`/app/contest/${fixture.contestId}/?$`))
  await expectNoHorizontalOverflow(page)
  await expectNoSeriousAccessibilityViolations(page)

  await page.getByRole('radio', { name: /alpha-marker/ }).click()
  const submissionResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes(`/api/v1/contests/${fixture.contestId}/submissions`) &&
      response.request().method() === 'POST'
  )
  await page.locator('[data-testid="submit-choice-sheet"]').click()
  const submissionResponse = await submissionResponsePromise
  expect(submissionResponse.status()).toBe(202)
  const submission = (await submissionResponse.json()) as {
    verdict: string
    score: number
  }
  expect(submission).toMatchObject({ verdict: 'accepted', score: 10 })
  await expect(page.getByText(/全卷通过/).first()).toBeVisible()

  await page.reload()
  await expect(page).toHaveURL(new RegExp(`/app/contest/${fixture.contestId}/?$`))
  await expect(page.getByRole('radio', { name: /alpha-marker/ })).toHaveAttribute(
    'aria-checked',
    'true'
  )

  await page.goto('/app/teacher')
  await expect(page).toHaveURL(/\/app\/?$/)

  await page.locator('[data-testid="logout-button"]').click()
  await expect(page).toHaveURL(/\/app\/login\/?$/)
  await expectNoSeriousAccessibilityViolations(page)
  expect(
    (await page.context().cookies()).some((cookie) => cookie.name === 'forge_session')
  ).toBe(false)
  expect(browserErrors).toEqual([])
})

test('student can edit, recover, and submit source from the real Monaco editor', async ({
  page,
  request,
}) => {
  const browserErrors = collectPageFailures(page)
  const fixture = await createContestWithChoiceProblem(request, {
    student: true,
    problemType: 'coding',
  })
  const marker = 'int main(){return 0;}'

  await uiLogin(page, fixture.username, STUDENT_PASSWORD)
  const card = page.locator(
    `[data-testid="contest-card"][data-contest-id="${fixture.contestId}"]`
  )
  await expect(card).toContainText(fixture.contestName)
  await card.locator('[data-testid="enter-contest"]').click()
  await expect(page).toHaveURL(new RegExp(`/app/contest/${fixture.contestId}/?$`))

  const editor = page.locator('.monaco-editor')
  await expect(editor).toBeVisible({ timeout: 20_000 })

  await page.getByRole('button', { name: '载入', exact: true }).click()
  const uploadDialog = page.getByRole('dialog', { name: '从本地载入代码' })
  await expect(uploadDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await page.keyboard.press('Escape')
  await expect(uploadDialog).toBeHidden()

  await editor.click({ position: { x: 240, y: 90 } })
  await page.keyboard.press('Control+A')
  await page.keyboard.insertText(marker)
  await expect(editor.locator('.view-lines')).toContainText(marker)

  // A real browser reload must hydrate the local draft into Monaco, not just
  // preserve an in-memory React value.
  await page.reload()
  await expect(page.locator('.monaco-editor')).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('.monaco-editor .view-lines')).toContainText(marker)

  const submitButton = page.getByRole('button', { name: '提交代码' }).first()
  await expect(submitButton).toBeEnabled()
  const submissionRequestPromise = page.waitForRequest(
    (candidate) =>
      candidate.url().includes(
        `/api/v1/contests/${fixture.contestId}/submissions`
      ) && candidate.method() === 'POST'
  )
  await submitButton.click()
  const submissionRequest = await submissionRequestPromise
  expect(submissionRequest.postDataJSON()).toMatchObject({
    problemID: fixture.problemId,
    language: 'cpp',
    code: marker,
  })
  expect(submissionRequest.headers()['idempotency-key']).toMatch(/^[a-f0-9-]{32,}$/)
  const submissionResponse = await submissionRequest.response()
  expect(submissionResponse?.status()).toBe(202)

  await expect(page.getByText('Queued').first()).toBeVisible()

  // A second submit must go through an explicit confirmation dialog. Scan that
  // nested state rather than only the base workspace.
  await page.getByRole('button', { name: '代码', exact: true }).click()
  await page.getByRole('button', { name: '提交代码', exact: true }).first().click()
  const confirmDialog = page.getByRole('dialog', { name: /再次提交/ })
  await expect(confirmDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await confirmDialog.getByRole('button', { name: '取消' }).click()
  await expect(confirmDialog).toBeHidden()

  await expectNoHorizontalOverflow(page)
  await expectNoSeriousAccessibilityViolations(page)
  expect(browserErrors).toEqual([])
})

test('administrator can open the teacher console and security controls', async ({
  page,
  request,
}) => {
  const browserErrors = collectPageFailures(page)

  const fixture = await createContestWithChoiceProblem(request, { student: false })
  const suffix = uniqueSuffix()
  const seededUsername = `e2e_teacher_${suffix}`
  const userResponse = await request.post('/api/v1/users', {
    data: {
      name: `E2E 考务演练 ${suffix}`,
      username: seededUsername,
      role: 'student',
      password: STUDENT_PASSWORD,
    },
  })
  expect(userResponse.status(), await userResponse.text()).toBe(201)
  const seededUser = (await userResponse.json()) as { id: string }
  const extensionResponse = await request.post(
    `/api/v1/contests/${fixture.contestId}/extensions`,
    { data: { userId: seededUser.id, minutes: 15 } }
  )
  expect(extensionResponse.status(), await extensionResponse.text()).toBe(200)

  await uiLogin(page, 'admin', ADMIN_PASSWORD)
  await expect(page.getByText(fixture.contestName).first()).toBeVisible()

  await page.goto('/app/teacher')
  await expect(page).toHaveURL(/\/app\/teacher\/?$/)
  await page.getByLabel('当前比赛').selectOption(fixture.contestId)
  await expect(page.getByRole('tablist', { name: /教师工作台分区/ })).toBeVisible()

  const tabs = page.getByRole('tab')
  const tabCount = await tabs.count()
  expect(tabCount).toBeGreaterThanOrEqual(6)
  for (let index = 0; index < tabCount; index += 1) {
    const tab = tabs.nth(index)
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByRole('tabpanel')).toBeVisible()
    await expectNoHorizontalOverflow(page)
    await expectNoSeriousAccessibilityViolations(page)
  }

  // 内置省份表：真实端点 + 只读视图（教师/管理员可见，管理员有操作列）。
  await page.getByRole('tab', { name: '省份与考号' }).click()
  await expect(page.getByRole('tabpanel', { name: '省份与考号' })).toBeVisible()
  await expect(
    page.getByRole('tabpanel', { name: '省份与考号' }).getByText('河南')
  ).toBeVisible()

  await tabs.first().click()
  await page.getByRole('button', { name: '新建比赛' }).click()
  const createDialog = page.getByRole('dialog', { name: '创建新比赛' })
  await expect(createDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await createDialog.getByRole('button', { name: '取消' }).click()
  await expect(createDialog).toBeHidden()

  await page.getByRole('button', { name: '导入比赛包' }).click()
  const importDialog = page.getByRole('dialog', { name: '导入 Forge 比赛包' })
  await expect(importDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await importDialog.getByRole('button', { name: '取消' }).click()
  await expect(importDialog).toBeHidden()

  await page.getByRole('tab').nth(1).click()
  await page.getByRole('button', { name: /智能准考证生成/ }).click()
  const seatDialog = page.getByRole('dialog', { name: /智能准考证/ })
  await expect(seatDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await page.keyboard.press('Escape')
  await expect(seatDialog).toBeHidden()

  await page.getByRole('tab', { name: /用户与编组/ }).click()
  const userRow = page.getByRole('row').filter({ hasText: seededUsername })
  await userRow.getByRole('button', { name: '重置密码' }).click()
  const resetDialog = page.getByRole('dialog', { name: '重置账号密码' })
  await expect(resetDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await page.keyboard.press('Escape')
  await expect(resetDialog).toBeHidden()

  await page.getByRole('tab', { name: /考务安全/ }).click()
  await page.getByRole('button', { name: /移除.*的加时/ }).first().click()
  const safetyDialog = page.getByRole('dialog', { name: /移除该考生的加时/ })
  await expect(safetyDialog).toBeVisible()
  await expectNoSeriousAccessibilityViolations(page)
  await page.keyboard.press('Escape')
  await expect(safetyDialog).toBeHidden()

  expect(browserErrors).toEqual([])
})
