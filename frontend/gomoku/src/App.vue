<script setup lang="ts">
import { ref, watch, nextTick, onMounted, computed } from 'vue'
import { useGameState } from './composables/useGameState'

// Declare useToast globally for TypeScript compilation since it is auto-imported by the Nuxt UI Vite plugin at build/runtime
declare const useToast: any

// Initialize game state
const state = useGameState()

// Game start banner state & watcher
const showStartBanner = ref(false)
const startBannerText = ref('')

watch(
  () => state.gameStatus.value,
  (newStatus, oldStatus) => {
    if (newStatus === 'playing' && oldStatus !== 'playing') {
      const selfColor = state.simulationRole.value
      if (selfColor === 'spectator') {
        startBannerText.value = '对局开始！当前身份为观战者'
      } else {
        const colorText = selfColor === 'black' ? '黑子' : '白子'
        const turnText = selfColor === 'black' ? '先手' : '后手'
        startBannerText.value = `对局开始！您执${colorText}（${turnText}）`
      }
      showStartBanner.value = true
      setTimeout(() => {
        showStartBanner.value = false
      }, 1000)
    }

    if (newStatus === 'ended' && oldStatus === 'playing') {
      const newWinner = state.winner.value
      if (newWinner) {
        const isWinnerSelf = state.simulationRole.value === newWinner
        const isPlayer = state.simulationRole.value !== 'spectator'
        const toastColor = isPlayer ? (isWinnerSelf ? 'success' : 'danger') : 'success'
        const winTitle = isPlayer ? (isWinnerSelf ? '🎉 恭喜获胜！' : '🔥 惜败，请再接再厉！') : '对局结束'

        toast.add({
          id: 'win-notification',
          title: winTitle,
          description: `【${getPlayerNameByColor(newWinner)}】执${newWinner === 'black' ? '黑子' : '白子'}获得胜利！`,
          icon: 'i-heroicons-trophy',
          color: toastColor,
          duration: 5000,
          actions: [
            {
              label: '确认',
              onClick: () => {
                state.resetGameState()
              }
            }
          ]
        })
      }
    }

    if (newStatus !== 'ended') {
      toast.remove('win-notification')
    }
  }
)

// Swapping helper: check if White player is the current user ("Me")
const isWhiteSelf = computed(() => {
  return state.playerWhite.value?.name === state.nickname.value
})

// Game started helper: whether game is currently playing or has ended
const isGameStarted = computed(() => {
  return state.gameStatus.value === 'playing' || state.gameStatus.value === 'ended'
})

// Force light theme
onMounted(() => {
  document.documentElement.classList.remove('dark')
  const observer = new MutationObserver(() => {
    if (document.documentElement.classList.contains('dark')) {
      document.documentElement.classList.remove('dark')
    }
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

// Nuxt UI Toast service (auto-imported by the Vite plugin)
const toast = useToast()

// Input states
const loginInput = ref(state.nickname.value)
const newRoomInput = ref('')
const chatInput = ref('')
const hoveredCell = ref<{ row: number; col: number }>({ row: 0, col: 0 })
const isHoverActive = ref(false)
const noTransition = ref(false)
const showSettingsPopover = ref(false)
const showResignDialog = ref(false)

const handleCellMouseEnter = (r: number, c: number) => {
  const isEmpty = state.board.value[r][c] === null
  const isPlaying = state.gameStatus.value === 'playing'
  const isPlayer = state.simulationRole.value !== 'spectator'
  const isMyTurn = state.turn.value === state.simulationRole.value

  if (isEmpty && isPlaying && isPlayer && isMyTurn) {
    if (!isHoverActive.value) {
      noTransition.value = true
      hoveredCell.value = { row: r, col: c }
      isHoverActive.value = true
      nextTick(() => {
        noTransition.value = false
      })
    } else {
      hoveredCell.value = { row: r, col: c }
      isHoverActive.value = true
    }
  } else {
    isHoverActive.value = false
  }
}

const handleCellClick = (r: number, c: number) => {
  state.placeStone(r, c)
  isHoverActive.value = false
}

// Sync login input if nickname changes
watch(state.nickname, newVal => {
  loginInput.value = newVal
})

// Watch manual refresh to trigger toast
watch(state.manualRefreshCount, newVal => {
  if (newVal !== null) {
    toast.add({
      title: '刷新成功',
      description: `已获取到 ${newVal} 个房间`,
      color: 'success',
      icon: 'i-heroicons-check-circle',
      duration: 1000
    })
    state.manualRefreshCount.value = null
  }
})

// Auto-scroll chat element
const chatFeedRef = ref<HTMLDivElement | null>(null)
watch(
  () => state.chatMessages.value.length,
  async () => {
    await nextTick()
    if (chatFeedRef.value) {
      chatFeedRef.value.scrollTop = chatFeedRef.value.scrollHeight
    }
  }
)

// Trigger handlers
const handleLogin = () => {
  if (loginInput.value.trim()) {
    state.saveNickname(loginInput.value)
  }
}

const handleCreateRoom = () => {
  if (newRoomInput.value.trim()) {
    state.createRoom(newRoomInput.value)
    newRoomInput.value = ''
  } else {
    toast.add({
      title: '提示',
      description: '请输入要创建的房间名称！',
      color: 'danger',
      icon: 'i-heroicons-exclamation-triangle'
    })
  }
}

const handleSendChat = () => {
  if (chatInput.value.trim()) {
    state.sendChat(chatInput.value)
    chatInput.value = ''
  }
}

const handleResign = () => {
  showResignDialog.value = true
}

// 15x15 Star Points checker
const isStarPoint = (r: number, c: number) => {
  return (
    (r === 3 && c === 3) ||
    (r === 3 && c === 11) ||
    (r === 7 && c === 7) ||
    (r === 11 && c === 3) ||
    (r === 11 && c === 11)
  )
}

// Helper to check if coordinate is part of the 5-in-a-row winning line
const isWinningCell = (r: number, c: number) => {
  return state.winningLine.value.some(coord => coord.row === r && coord.col === c)
}

// Helper to resolve player name by stone color
const getPlayerNameByColor = (color: 'black' | 'white') => {
  if (color === 'black') {
    return state.playerBlack.value?.name || '黑方玩家'
  }
  return state.playerWhite.value?.name || '白方玩家'
}

// Helper to check if coordinate is the most recently placed stone
const isLastMove = (r: number, c: number) => {
  if (state.history.value.length === 0) return false
  const last = state.history.value[state.history.value.length - 1]
  return last.row === r && last.col === c
}
</script>

<template>
  <UApp :toaster="{ position: 'top-center' }">
    <div
      class="flex flex-col min-h-screen relative z-10 font-sans antialiased transition-colors duration-500"
      :class="[state.currentView.value === 'room' ? 'bg-slate-300' : 'bg-slate-50/40']"
    >
      <!-- Full-screen Vignette Lens Dark Corner Effect (Only in Room battle View) -->
      <div
        v-if="state.currentView.value === 'room'"
        class="pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(circle_at_center,transparent_30%,rgba(0,0,0,0.32)_100%)]"
      ></div>

      <main
        class="flex-grow max-w-7xl w-full mx-auto p-6 flex flex-col justify-start relative z-10"
      >
        <!-- ==========================================================================
             VIEW 1: LOGIN / NICKNAME ENTRANCE
             ========================================================================== -->
        <section
          v-if="state.currentView.value === 'login'"
          class="flex-grow flex items-center justify-center py-10"
        >
          <UCard
            class="w-full max-w-sm shadow-xl border border-slate-100 rounded-2xl p-4 bg-white/90 backdrop-blur-md"
          >
            <h2
              class="text-3xl font-extrabold text-slate-800 text-center tracking-tight mb-6 bg-gradient-to-r from-slate-700 via-slate-900 to-zinc-900 bg-clip-text text-transparent"
            >
              五子棋
            </h2>

            <form class="space-y-5" @submit.prevent="handleLogin">
              <div class="space-y-2">
                <label class="text-sm font-bold text-slate-700 block" for="username">
                  请输入您的昵称
                </label>
                <UInput
                  id="username"
                  v-model="loginInput"
                  type="text"
                  size="lg"
                  icon="i-heroicons-user"
                  placeholder="请输入您的昵称"
                  required
                  maxlength="20"
                  autocomplete="off"
                  class="rounded-xl w-full"
                />
              </div>

              <UButton
                type="submit"
                block
                size="lg"
                color="primary"
                class="font-bold shadow-md hover:shadow-lg shadow-slate-200 hover:shadow-slate-300 transition-all rounded-xl cursor-pointer"
                trailing-icon="i-heroicons-arrow-right-20-solid"
              >
                进入游戏
              </UButton>
            </form>
          </UCard>
        </section>

        <!-- ==========================================================================
             VIEW 2: LOBBY / ROOM BROWSER
             ========================================================================== -->
        <section v-else-if="state.currentView.value === 'lobby'" class="space-y-8 py-6">
          <div
            class="flex items-center justify-between flex-wrap gap-4 border-b border-slate-100 pb-6"
          >
            <div class="space-y-1">
              <h2 class="text-2xl font-extrabold text-slate-800 tracking-tight">游戏大厅</h2>
              <p class="text-sm text-slate-500">以下是当前的棋局，您可以随时创建或加入</p>
            </div>

            <div class="flex items-center gap-3">
              <span class="text-sm font-bold text-slate-700 flex items-center gap-1.5">
                <UAvatar
                  :text="state.nickname.value ? state.nickname.value[0].toUpperCase() : '?'"
                  size="xs"
                />
                {{ state.nickname.value }}
              </span>
              <UButton
                color="neutral"
                variant="outline"
                size="sm"
                class="font-bold rounded-xl border-slate-200 hover:bg-slate-50"
                icon="i-heroicons-arrow-right-on-rectangle"
                @click="state.logout"
              >
                离开
              </UButton>
            </div>
          </div>

          <div class="flex items-center justify-between flex-wrap gap-4">
            <UButton
              color="neutral"
              variant="subtle"
              size="md"
              class="font-bold rounded-xl shadow-sm hover:scale-[1.02] active:scale-[0.98] transition-all cursor-pointer"
              icon="i-heroicons-arrow-path"
              @click="state.refreshRooms"
            >
              刷新列表
            </UButton>

            <div class="flex items-center gap-3">
              <UInput
                v-model="newRoomInput"
                type="text"
                size="md"
                placeholder="输入一个新房间的名字"
                maxlength="20"
                @keyup.enter="handleCreateRoom"
                class="w-64"
              />
              <UButton
                color="primary"
                size="md"
                class="font-bold rounded-xl cursor-pointer"
                icon="i-heroicons-plus"
                @click="handleCreateRoom"
              >
                创建房间
              </UButton>
            </div>
          </div>

          <div v-if="state.rooms.length === 0" class="flex flex-col items-center justify-center py-16 px-4 border border-dashed border-slate-200 rounded-2xl bg-white/50 backdrop-blur-sm">
            <span class="text-4xl mb-3 opacity-60">🎮</span>
            <p class="text-sm font-bold text-slate-400 tracking-wide">空空如也</p>
            <p class="text-xs text-slate-400/80 mt-1">当前大厅空无一人，输入名称创建一个新房间吧！</p>
          </div>

          <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
            <UCard
              v-for="room in state.rooms"
              :key="room.id"
              class="hover:shadow-xl hover:border-emerald-200 transition-all duration-300 cursor-pointer border border-slate-100 rounded-2xl flex flex-col justify-between"
              @click="state.joinRoom(room)"
            >
              <template #header>
                <div class="flex items-center justify-between">
                  <UBadge
                    :color="
                      room.status === 'waiting'
                        ? 'neutral'
                        : room.status === 'playing'
                          ? 'success'
                          : 'warning'
                    "
                    variant="subtle"
                    size="sm"
                    class="font-bold rounded-md"
                  >
                    {{
                      room.status === 'waiting'
                        ? '等待中'
                        : room.status === 'playing'
                          ? '对局中'
                          : '已满员'
                    }}
                  </UBadge>
                  <span class="text-xs font-semibold text-slate-400 flex items-center gap-1">
                    👥 {{ room.playerCount }}/{{ room.maxPlayers }}
                  </span>
                </div>
              </template>

              <h3 class="text-lg font-bold text-slate-800 line-clamp-1 mb-2">{{ room.name }}</h3>

              <template #footer>
                <div class="text-xs text-slate-400">房主：{{ room.creatorName }}</div>
              </template>
            </UCard>
          </div>
        </section>

        <!-- ==========================================================================
             VIEW 3: ROOM / GAME BOARD VIEW
             ========================================================================== -->
        <div
          v-else-if="state.currentView.value === 'room'"
          class="space-y-6 flex-grow flex flex-col justify-center"
        >
          <!-- Room Header with Settings -->
          <div
            class="flex items-center justify-between flex-wrap gap-4 border-b border-slate-400/40 pb-4 mb-2 z-10 relative"
          >
            <div class="space-y-1">
              <h2 class="text-2xl font-extrabold text-slate-800 tracking-tight">
                房间：{{ state.activeRoom.value?.name }}
              </h2>
              <p class="text-sm text-slate-600">
                房间ID: {{ state.activeRoom.value?.id }} | 状态:
                <span 
                  class="font-bold"
                  :class="[state.gameStatus.value === 'playing' ? 'text-emerald-600' : 'text-slate-600']"
                >
                  {{ state.gameStatus.value === 'playing' ? '对局中' : '等待中' }}
                </span>
              </p>
            </div>

            <!-- Settings Popover for Host (Gear Icon) -->
            <div
              v-if="state.activeRoom.value?.host?.name === state.nickname.value"
              class="relative z-30"
            >
              <UButton
                color="neutral"
                variant="ghost"
                icon="i-heroicons-cog-6-tooth"
                size="md"
                class="rounded-full hover:bg-slate-200/50 transition-colors"
                @click="showSettingsPopover = !showSettingsPopover"
              />

              <!-- Floating Settings Popup Card -->
              <transition name="popover-fade">
                <div
                  v-if="showSettingsPopover"
                  class="absolute right-0 mt-2 w-60 bg-white/95 backdrop-blur-md rounded-2xl border border-slate-200 shadow-2xl p-4 space-y-4"
                >
                  <h4
                    class="text-sm font-bold text-slate-800 border-b border-slate-100 pb-2 flex items-center gap-1.5"
                  >
                    ⚙️ 房间设置
                  </h4>

                  <div class="space-y-3">
                    <!-- Auto Join Spectator Toggle -->
                    <div class="flex items-center justify-between">
                      <span class="text-xs font-semibold text-slate-700">观战自动补位</span>
                      <USwitch
                        :model-value="state.activeRoom.value?.config?.autoJoinSpectator"
                        @update:model-value="
                          (val: boolean) =>
                            state.updateRoomConfig({
                              ...state.activeRoom.value?.config,
                              autoJoinSpectator: val
                            })
                        "
                      />
                    </div>

                    <!-- Disable Chat Toggle -->
                    <div class="flex items-center justify-between">
                      <span class="text-xs font-semibold text-slate-700">禁用聊天区</span>
                      <USwitch
                        :model-value="state.activeRoom.value?.config?.disableChat"
                        @update:model-value="
                          (val: boolean) =>
                            state.updateRoomConfig({
                              ...state.activeRoom.value?.config,
                              disableChat: val
                            })
                        "
                      />
                    </div>

                    <!-- Color Mode Selector Toggle -->
                    <div class="flex items-center justify-between pt-1 border-t border-slate-100">
                      <span class="text-xs font-semibold text-slate-700">随机执子分配</span>
                      <USwitch
                        :model-value="state.activeRoom.value?.config?.colorMode === 'random'"
                        @update:model-value="
                          (val: boolean) =>
                            state.updateRoomConfig({
                              ...state.activeRoom.value?.config,
                              colorMode: val ? 'random' : 'alternating'
                            })
                        "
                      />
                    </div>
                  </div>
                </div>
              </transition>
            </div>

            <!-- Settings Popover for Non-Hosts (Info Icon) -->
            <div v-else class="relative z-30">
              <UButton
                color="neutral"
                variant="ghost"
                icon="i-heroicons-information-circle"
                size="md"
                class="rounded-full hover:bg-slate-200/50 transition-colors"
                @click="showSettingsPopover = !showSettingsPopover"
              />

              <!-- Floating Settings Display Card -->
              <transition name="popover-fade">
                <div
                  v-if="showSettingsPopover"
                  class="absolute right-0 mt-2 w-52 bg-white/95 backdrop-blur-md rounded-2xl border border-slate-200 shadow-2xl p-4 space-y-3"
                >
                  <h4
                    class="text-sm font-bold text-slate-800 border-b border-slate-100 pb-2 flex items-center gap-1.5"
                  >
                    当前房间设置
                  </h4>
                  <div class="space-y-2 text-xs font-semibold text-slate-600">
                    <div class="flex justify-between">
                      <span>观战自动补位：</span>
                      <span :class="[state.activeRoom.value?.config?.autoJoinSpectator ? 'text-emerald-600' : 'text-slate-500']">
                        {{ state.activeRoom.value?.config?.autoJoinSpectator ? '开启' : '关闭' }}
                      </span>
                    </div>
                    <div class="flex justify-between">
                      <span>聊天区：</span>
                      <span :class="[!state.activeRoom.value?.config?.disableChat ? 'text-emerald-600' : 'text-slate-500']">
                        {{ state.activeRoom.value?.config?.disableChat ? '禁用' : '启用' }}
                      </span>
                    </div>
                    <div class="flex justify-between">
                      <span>执子分配：</span>
                      <span class="text-slate-800">
                        {{
                          state.activeRoom.value?.config?.colorMode === 'random'
                            ? '随机分配'
                            : '黑白轮换'
                        }}
                      </span>
                    </div>
                  </div>
                </div>
              </transition>
            </div>
          </div>

          <section class="grid grid-cols-1 xl:grid-cols-12 gap-8 items-start">
            <!-- Left Column: The Flat Go Board -->
            <div class="xl:col-span-7 flex flex-col items-center">
              <div class="board-inner">
                <div class="board-grid-wrapper">
                  <!-- Gliding hover placement indicator (four-corner L-brackets) -->
                  <div
                    class="hover-corner-indicator"
                    :class="{
                      'is-active': isHoverActive,
                      'no-transition': noTransition
                    }"
                    :style="{
                      left: `${(hoveredCell.col * 100) / state.boardSize}%`,
                      top: `${(hoveredCell.row * 100) / state.boardSize}%`,
                      width: `${100 / state.boardSize}%`,
                      height: `${100 / state.boardSize}%`
                    }"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      fill="none"
                      xmlns="http://www.w3.org/2000/svg"
                      class="w-full h-full"
                    >
                      <!-- Top Left Corner -->
                      <path
                        d="M6 2H2V6"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Top Right Corner -->
                      <path
                        d="M18 2H22V6"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Bottom Left Corner -->
                      <path
                        d="M6 22H2V18"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                      <!-- Bottom Right Corner -->
                      <path
                        d="M18 22H22V18"
                        stroke="currentColor"
                        stroke-width="1.2"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </div>

                  <div class="board-grid" @mouseleave="isHoverActive = false">
                    <div v-for="r in state.boardSize" :key="r" class="board-row">
                      <div
                        v-for="c in state.boardSize"
                        :key="c"
                        class="board-cell"
                        @click="handleCellClick(r - 1, c - 1)"
                        @mouseenter="handleCellMouseEnter(r - 1, c - 1)"
                      >
                        <!-- Star point coordinates overlay -->
                        <div v-if="isStarPoint(r - 1, c - 1)" class="star-point-dot"></div>

                        <!-- Placed spherical 3D stone -->
                        <div
                          v-if="state.board.value[r - 1][c - 1] !== null"
                          :class="[
                            'stone',
                            state.board.value[r - 1][c - 1],
                            'stone-placed-animation'
                          ]"
                        >
                          <!-- Last move indicator (subtle glowing red center dot) -->
                          <div v-if="isLastMove(r - 1, c - 1)" class="last-move-indicator"></div>
                        </div>

                        <!-- 5-in-a-row Win highlight overlay (perfectly centered to board-cell) -->
                        <div v-if="isWinningCell(r - 1, c - 1)" class="winning-highlight"></div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Column: Symmetrical Match Hub Card powered by Nuxt UI UCard -->
            <div class="xl:col-span-5 w-full">
              <UCard
                :ui="{
                  root: 'match-hub-card flex flex-col shadow-xl border border-slate-100 rounded-3xl bg-white/90 backdrop-blur-md overflow-hidden',
                  body: 'flex-grow flex flex-col min-h-0 p-4 sm:p-6',
                  footer: 'p-4 sm:p-6 border-t border-slate-100'
                }"
              >
                <div class="space-y-6 flex-grow flex flex-col min-h-0">
                  <div class="flex items-center justify-between gap-2">
                    <!-- Black Player -->
                    <div
                      :class="[
                        'flex-1 min-w-0 grid grid-cols-[auto_1fr] gap-x-1.5 sm:gap-x-2 gap-y-1 items-center p-2 sm:p-3 rounded-2xl border transition-all duration-300',
                        state.turn.value === 'black' && state.gameStatus.value === 'playing'
                          ? 'border-slate-700 bg-slate-100/30 ring-2 ring-slate-700/20 shadow-sm'
                          : 'border-slate-200 bg-slate-50/50 shadow-sm',
                        isWhiteSelf ? 'order-3' : 'order-1'
                      ]"
                    >
                      <UAvatar
                        :text="
                          state.playerBlack.value?.name
                            ? state.playerBlack.value.name[0].toUpperCase()
                            : '?'
                        "
                        size="sm"
                        class="flex-shrink-0"
                      />
                      <span
                        class="text-sm font-bold text-slate-800 truncate flex items-center gap-1"
                      >
                        {{ state.playerBlack.value?.name || '等待加入...' }}
                        <span
                          v-if="state.playerBlack.value?.isOffline"
                          class="text-[10px] font-extrabold text-red-500 bg-red-100 px-1.5 py-0.5 rounded-full border border-red-200 animate-pulse"
                        >
                          离线
                        </span>
                      </span>
                      <span
                        class="inline-block w-3 h-3 rounded-full bg-gradient-to-br from-slate-700 to-slate-900 border border-slate-950 shadow-sm justify-self-center"
                      ></span>
                      <div class="flex items-center gap-1.5">
                        <UBadge
                          v-if="isGameStarted"
                          color="neutral"
                          variant="solid"
                          size="sm"
                          class="font-bold rounded-md shadow-sm"
                        >
                          执黑
                        </UBadge>
                        <UBadge
                          v-else
                          :color="state.playerBlack.value?.isReady ? 'success' : 'neutral'"
                          :variant="state.playerBlack.value?.isReady ? 'solid' : 'subtle'"
                          size="sm"
                          :class="[
                            'font-bold rounded-md shadow-sm',
                            !state.playerBlack.value?.isReady
                              ? 'text-slate-500 bg-slate-100 border border-slate-200'
                              : ''
                          ]"
                        >
                          {{ state.playerBlack.value?.isReady ? '已准备' : '等待中' }}
                        </UBadge>
                        <span
                          v-if="state.playerBlack.value?.name === state.nickname.value"
                          class="text-xs font-bold text-sky-500"
                        >
                          (我)
                        </span>
                      </div>
                    </div>

                    <div class="text-sm font-black text-slate-400 opacity-60 order-2 flex-shrink-0">
                      VS
                    </div>

                    <!-- White Player -->
                    <div
                      :class="[
                        'flex-1 min-w-0 grid grid-cols-[auto_1fr] gap-x-1.5 sm:gap-x-2 gap-y-1 items-center p-2 sm:p-3 rounded-2xl border transition-all duration-300',
                        state.turn.value === 'white' && state.gameStatus.value === 'playing'
                          ? 'border-purple-500 bg-purple-50/30 ring-2 ring-purple-500/20 shadow-sm'
                          : 'border-slate-200 bg-slate-50/50 shadow-sm',
                        isWhiteSelf ? 'order-1' : 'order-3'
                      ]"
                    >
                      <UAvatar
                        :text="
                          state.playerWhite.value?.name
                            ? state.playerWhite.value.name[0].toUpperCase()
                            : '?'
                        "
                        size="sm"
                        class="flex-shrink-0"
                      />
                      <span
                        class="text-sm font-bold text-slate-800 truncate flex items-center gap-1"
                      >
                        {{ state.playerWhite.value?.name || '等待加入...' }}
                        <span
                          v-if="state.playerWhite.value?.isOffline"
                          class="text-[10px] font-extrabold text-red-500 bg-red-100 px-1.5 py-0.5 rounded-full border border-red-200 animate-pulse"
                        >
                          离线
                        </span>
                      </span>
                      <span
                        class="inline-block w-3 h-3 rounded-full bg-gradient-to-br from-slate-50 to-slate-200 border border-slate-300 shadow-sm justify-self-center"
                      ></span>
                      <div class="flex items-center gap-1.5">
                        <UBadge
                          v-if="isGameStarted"
                          color="neutral"
                          variant="outline"
                          size="sm"
                          class="font-bold rounded-md shadow-sm bg-white border-slate-300 text-slate-700"
                        >
                          执白
                        </UBadge>
                        <UBadge
                          v-else
                          :color="state.playerWhite.value?.isReady ? 'success' : 'neutral'"
                          :variant="state.playerWhite.value?.isReady ? 'solid' : 'subtle'"
                          size="sm"
                          :class="[
                            'font-bold rounded-md shadow-sm',
                            !state.playerWhite.value?.isReady
                              ? 'text-slate-500 bg-slate-100 border border-slate-200'
                              : ''
                          ]"
                        >
                          {{ state.playerWhite.value?.isReady ? '已准备' : '等待中' }}
                        </UBadge>
                        <span
                          v-if="state.playerWhite.value?.name === state.nickname.value"
                          class="text-xs font-bold text-sky-500"
                        >
                          (我)
                        </span>
                      </div>
                    </div>
                  </div>

                  <!-- Spectators bar -->
                  <div
                    class="hidden md:flex items-center gap-2 text-xs text-slate-500 bg-slate-50 border border-slate-100 p-2.5 rounded-xl"
                  >
                    <span class="font-bold">👥 观战中 ({{ state.spectators.value.length }})：</span>
                    <span class="font-semibold text-slate-700 truncate max-w-[280px]">
                      {{ state.spectators.value.map(s => s.name).join(', ') || '暂无观战人员' }}
                    </span>
                  </div>

                  <!-- Live Chat Box Container -->
                  <div
                    class="hidden md:flex flex-col border border-slate-100 rounded-2xl overflow-hidden bg-slate-50/50 flex-grow min-h-0 h-[320px] xl:h-auto"
                  >
                    <div
                      ref="chatFeedRef"
                      class="flex-grow p-4 overflow-y-auto space-y-3 flex flex-col"
                    >
                      <div
                        v-for="msg in state.chatMessages.value"
                        :key="msg.id"
                        :class="[
                          'flex flex-col max-w-[80%] rounded-2xl px-3 py-2 text-sm leading-relaxed shadow-sm',
                          msg.isSystem
                            ? 'self-center max-w-[90%] text-xs italic bg-slate-100 text-slate-500 rounded-xl py-1 border-0 shadow-none'
                            : msg.senderName === state.nickname.value
                              ? 'self-end bg-emerald-600 text-white rounded-tr-none'
                              : 'self-start bg-white text-slate-800 border border-slate-100 rounded-tl-none'
                        ]"
                      >
                        <span v-if="!msg.isSystem" class="text-[10px] font-bold opacity-60 mb-0.5">
                          {{ msg.senderName }} ({{ msg.timestamp }})
                        </span>
                        <span class="break-all">{{ msg.text }}</span>
                      </div>
                    </div>

                    <!-- Chat Input bar -->
                    <form
                      class="flex border-t border-slate-100 bg-white"
                      @submit.prevent="handleSendChat"
                    >
                      <input
                        v-model="chatInput"
                        type="text"
                        class="flex-grow border-0 px-4 py-3 outline-none text-sm text-slate-800 disabled:bg-slate-50 disabled:text-slate-400 disabled:italic rounded-bl-2xl"
                        :placeholder="
                          state.simulationRole.value === 'spectator' ? '观棋不语真君子' : '说点什么'
                        "
                        :disabled="state.simulationRole.value === 'spectator'"
                        maxlength="100"
                      />
                      <UButton
                        type="submit"
                        variant="ghost"
                        color="neutral"
                        size="sm"
                        :disabled="state.simulationRole.value === 'spectator' || !chatInput.trim()"
                        class="font-bold px-4 rounded-none rounded-br-2xl"
                      >
                        发送
                      </UButton>
                    </form>
                  </div>
                </div>

                <!-- Dashboard Symmetrical Actions Panel powered by Nuxt UI UButton -->
                <template #footer>
                  <div class="grid grid-cols-2 gap-3 sm:gap-4">
                    <!-- 悔棋 -->
                    <UButton
                      block
                      color="neutral"
                      variant="solid"
                      size="md"
                      class="font-bold rounded-xl shadow-sm border border-slate-200"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.gameStatus.value !== 'playing' ||
                        state.history.value.length === 0 ||
                        state.activeRoom.value?.retractRequester !== '' ||
                        state.retractCooldown.value > 0
                      "
                      @click="state.retractMove"
                    >
                      {{
                        state.retractCooldown.value > 0
                          ? `悔棋 (${state.retractCooldown.value}s)`
                          : '悔棋'
                      }}
                    </UButton>

                     <!-- 认输 -->
                    <UButton
                      block
                      color="danger"
                      variant="solid"
                      size="md"
                      class="font-bold rounded-xl shadow-sm shadow-red-50 hover:bg-red-600 active:bg-red-700 hover:scale-[1.02] active:scale-[0.98] transition-all cursor-pointer"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.gameStatus.value !== 'playing'
                      "
                      @click="handleResign"
                    >
                      认输
                    </UButton>

                    <!-- 准备 -->
                    <UButton
                      block
                      :color="state.gameStatus.value === 'playing' ? 'success' : 'primary'"
                      :variant="
                        state.gameStatus.value === 'playing'
                          ? 'soft'
                          : state.simulationRole.value !== 'spectator' &&
                              ((state.simulationRole.value === 'black' &&
                                state.playerBlack.value?.isReady) ||
                                (state.simulationRole.value === 'white' &&
                                  state.playerWhite.value?.isReady))
                            ? 'soft'
                            : 'solid'
                      "
                      size="md"
                      class="font-bold rounded-xl shadow-md"
                      :class="[state.gameStatus.value === 'playing' ? 'shadow-emerald-50' : 'shadow-slate-100']"
                      :disabled="
                        state.simulationRole.value === 'spectator' ||
                        state.gameStatus.value === 'playing'
                      "
                      @click="state.toggleReady"
                    >
                      {{
                        state.gameStatus.value === 'playing'
                          ? '对局中'
                          : state.simulationRole.value !== 'spectator' &&
                              ((state.simulationRole.value === 'black' &&
                                state.playerBlack.value?.isReady) ||
                                (state.simulationRole.value === 'white' &&
                                  state.playerWhite.value?.isReady))
                            ? '取消准备'
                            : '准备'
                      }}
                    </UButton>

                    <!-- 离开房间 -->
                    <UButton
                      block
                      color="neutral"
                      variant="outline"
                      size="md"
                      class="font-bold rounded-xl border border-slate-200 hover:bg-slate-50"
                      :disabled="
                        state.simulationRole.value !== 'spectator' &&
                        state.gameStatus.value === 'playing'
                      "
                      @click="state.leaveRoom"
                    >
                      离开房间
                    </UButton>
                  </div>
                </template>
              </UCard>
            </div>
          </section>
        </div>
      </main>

      <!-- Game Start Screen Overlay Banner (1.5s auto fade out) -->
      <transition name="banner-fade">
        <div
          v-if="showStartBanner"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/45 backdrop-blur-[2px] pointer-events-none select-none"
        >
          <div
            class="bg-slate-800/95 text-white px-8 py-5 rounded-3xl shadow-2xl text-2xl font-black tracking-wider border border-slate-700/40 scale-100 transform transition-all duration-300 flex flex-col items-center gap-2"
          >
            <span class="text-3xl animate-bounce">⚔️</span>
            <span>{{ startBannerText }}</span>
          </div>
        </div>
      </transition>

      <!-- Retract Consent Dialog (Custom CSS Modal) -->
      <transition name="modal-fade">
        <div
          v-if="state.showRetractDialog.value"
          class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/25 p-4 select-none"
        >
          <div
            class="w-full max-w-sm bg-white rounded-3xl shadow-2xl border border-slate-100 p-6 space-y-6 transform scale-100 transition-all duration-300"
          >
            <div class="flex items-center gap-3">
              <div class="w-12 h-12 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center flex-shrink-0">
                <UIcon name="i-heroicons-arrow-path" class="w-6 h-6" />
              </div>
              <div>
                <h3 class="font-extrabold text-lg text-slate-800">对方请求悔棋</h3>
                <p class="text-xs text-slate-400">请选择是否同意对方的悔棋申请</p>
              </div>
            </div>

            <div
              class="bg-slate-50 border border-slate-100 p-4 rounded-2xl text-sm font-semibold text-slate-700 leading-relaxed text-center"
            >
              【{{ state.retractRequesterName.value }}】请求撤回上一步落子。
            </div>

            <div class="grid grid-cols-2 gap-4">
              <button
                class="w-full py-3 rounded-2xl font-bold bg-slate-100 hover:bg-slate-200 text-slate-700 transition-all shadow-sm cursor-pointer"
                @click="state.respondRetract(false)"
              >
                拒绝
              </button>
              <button
                class="w-full py-3 rounded-2xl font-bold bg-emerald-600 hover:bg-emerald-700 text-white transition-all shadow-md shadow-emerald-100 cursor-pointer"
                @click="state.respondRetract(true)"
              >
                同意悔棋
              </button>
            </div>
          </div>
        </div>
      </transition>

      <!-- Resign Consent Dialog (Custom CSS Modal) -->
      <transition name="modal-fade">
        <div
          v-if="showResignDialog"
          class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/25 p-4 select-none"
        >
          <div
            class="w-full max-w-sm bg-white rounded-3xl shadow-2xl border border-slate-100 p-6 space-y-6 transform scale-100 transition-all duration-300"
          >
            <div class="flex items-center gap-3">
              <div class="w-12 h-12 rounded-full bg-red-50 text-red-600 flex items-center justify-center flex-shrink-0">
                <UIcon name="i-heroicons-flag" class="w-6 h-6" />
              </div>
              <div>
                <h3 class="font-extrabold text-lg text-slate-800">确认认输吗？</h3>
                <p class="text-xs text-slate-400">认输后将直接判定对方获得本局胜利</p>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <button
                class="w-full py-3 rounded-2xl font-bold bg-slate-100 hover:bg-slate-200 text-slate-700 transition-all shadow-sm cursor-pointer"
                @click="showResignDialog = false"
              >
                继续对局
              </button>
              <button
                class="w-full py-3 rounded-2xl font-bold bg-red-600 hover:bg-red-700 text-white transition-all shadow-md shadow-red-100 cursor-pointer"
                @click="
                  state.resignGame();
                  showResignDialog = false;
                "
              >
                确认认输
              </button>
            </div>
          </div>
        </div>
      </transition>
    </div>
  </UApp>
</template>

<style>
/* Game Start Banner Transition */
.banner-fade-enter-active,
.banner-fade-leave-active {
  transition:
    opacity 0.4s ease,
    transform 0.4s ease;
}
.banner-fade-enter-from,
.banner-fade-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

/* Retract Modal Transition */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.3s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

/* Popover Fade Transition */
.popover-fade-enter-active,
.popover-fade-leave-active {
  transition:
    opacity 0.18s ease-out,
    transform 0.18s ease-out;
}
.popover-fade-enter-from,
.popover-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
