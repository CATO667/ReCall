import axios, { type AxiosResponse } from 'axios'

const service = axios.create({
  baseURL: import.meta.env.VITE_GLOB_API_URL,
})

service.interceptors.request.use(
  (config) => config,
  (error) => Promise.reject(error.response),
)

service.interceptors.response.use(
  (response: AxiosResponse): AxiosResponse => {
    if (response.status === 200)
      return response

    if (response.status === 401)
      window.location.reload()

    throw new Error(response.status.toString())
  },
  (error) => Promise.reject(error),
)

export default service
