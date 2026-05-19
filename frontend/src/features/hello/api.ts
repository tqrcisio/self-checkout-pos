import { Hello as WailsHello } from '@/wailsjs/go/main/App'

export function hello(name: string): Promise<string> {
  return WailsHello(name)
}
