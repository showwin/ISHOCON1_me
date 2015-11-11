require 'sinatra/base'
require 'mysql2'
require 'mysql2-cs-bind'
require 'erubis'
require 'logger'
require 'redis'

module Ishocon1
  class AuthenticationError < StandardError; end
  class PermissionDenied < StandardError; end
end

class Ishocon1::WebApp < Sinatra::Base
  session_secret = ENV['ISHOCON1_SESSION_SECRET'] || 'showwin_happy'
  use Rack::Session::Cookie, key: 'rack.session', secret: session_secret
  set :erb, escape_html: true
  set :public_folder, File.expand_path('../public', __FILE__)
  set :protection, true
  Log = Logger.new('dev.log')

  helpers do
    def config
      @config ||= {
        db: {
          host: ENV['ISHOCON1_DB_HOST'] || 'localhost',
          port: ENV['ISHOCON1_DB_PORT'] && ENV['ISHOCON1_DB_PORT'].to_i,
          username: ENV['ISHOCON1_DB_USER'] || 'ishocon',
          password: ENV['ISHOCON1_DB_PASSWORD'] || 'ishocon',
          database: ENV['ISHOCON1_DB_NAME'] || 'ishocon1'
        }
      }
    end

    def redis
      return Thread.current[:ishocon1_redis] if Thread.current[:ishocon1_redis]
      client = Redis.new(:path => '/tmp/redis.sock')
      Thread.current[:ishocon1_redis] = client
      client
    end

    def db
      return Thread.current[:ishocon1_db] if Thread.current[:ishocon1_db]
      client = Mysql2::Client.new(
        host: config[:db][:host],
        port: config[:db][:port],
        username: config[:db][:username],
        password: config[:db][:password],
        database: config[:db][:database],
        reconnect: true
      )
      client.query_options.merge!(symbolize_keys: true)
      Thread.current[:ishocon1_db] = client
      client
    end

    def authenticate(email, password)
      user = db.xquery('SELECT * FROM users WHERE email = ?', email).first
      fail Ishocon1::AuthenticationError unless user[:password] == password
      session[:user_id] = user[:id]
    end

    def authenticated!
      fail Ishocon1::PermissionDenied unless session[:user_id]
    end

    def current_user
      db.xquery('SELECT * FROM users WHERE id = ?', session[:user_id]).first
    end

    def update_last_login(user_id)
      db.xquery('UPDATE users SET last_login = ? WHERE id = ?', Time.now, user_id)
    end

    def buy_product(product_id, user_id)
      db.xquery('INSERT INTO histories (product_id, user_id, created_at) VALUES (?, ?, ?)', \
        product_id, user_id, Time.now)
    end

    def already_bought?(product_id)
      return false unless session[:user_id]
      count = db.xquery('SELECT count(*) as count FROM histories WHERE product_id = ? AND user_id = ?', \
                        product_id, session[:user_id]).first[:count]
      count > 0
    end

    def create_comment(product_id, user_id, content)
      # コメント総件数更新
      redis.incr(product_id.to_s+'cnt')

      # コメント登録
      user_name = redis.get(user_id.to_s+'nm')
      key = product_id.to_s+'c'
      redis.rpop(key) if redis.LLEN(key) == 5
      redis.lpush(key, user_name+',,'+content[0..25])
      #db.xquery('INSERT INTO comments (product_id, user_id, content, created_at) VALUES (?, ?, ?, ?)', \
      #  product_id, user_id, content, Time.now)
    end
  end

  error Ishocon1::AuthenticationError do
    session[:user_id] = nil
    halt 401, erb(:login, layout: false, locals: { message: 'ログインに失敗しました' })
  end

  error Ishocon1::PermissionDenied do
    halt 403, erb(:login, layout: false, locals: { message: '先にログインをしてください' })
  end

  get '/login' do
    session.clear
    erb :login, layout: false, locals: { message: 'ECサイトで爆買いしよう！！！！' }
  end

  post '/login' do
    authenticate(params['email'], params['password'])
    
    # index と一緒
    page = params[:page].to_i || 0

    result = db.xquery("SELECT id FROM products ORDER BY id DESC LIMIT 50 OFFSET #{page * 50}")
    product_ids = result.map { |r| r[:id] }
    query = <<SQL
SELECT id, name, image_path, price, LEFT(description, 70) as description
FROM products
WHERE id in (?)
ORDER BY id DESC
SQL
    products = db.xquery(query, product_ids)
    
    erb :index, locals: { products: products }
  end

  get '/logout' do
    session[:user_id] = nil
    session.clear
    erb :login, layout: false, locals: { message: 'ECサイトで爆買いしよう！！！！' }
  end

  get '/' do
    page = params[:page].to_i || 0

    result = db.xquery("SELECT id FROM products ORDER BY id DESC LIMIT 50 OFFSET #{page * 50}")
    product_ids = result.map { |r| r[:id] }
    query = <<SQL
SELECT id, name, image_path, price, LEFT(description, 70) as description
FROM products
WHERE id in (?)
ORDER BY id DESC
SQL
    products = db.xquery(query, product_ids)
    
    #cmt_query = <<SQL
#SELECT LEFT(content, 26), user_id
#FROM comments
#WHERE product_id = ?
#ORDER BY created_at DESC
#LIMIT 5
#SQL
    #cmt_count_query = 'SELECT count(*) as count FROM comments WHERE product_id = ?'

    erb :index, locals: { products: products }
  end

  get '/users/:user_id' do
    query = <<SQL
SELECT id
FROM histories
WHERE user_id = ?
SQL
    result = db.xquery(query, params[:user_id])
    his_ids = result.map { |r| r[:id] }

    products_query = <<SQL
SELECT p.id, p.name, p.description, p.image_path, p.price, h.created_at
FROM histories as h
LEFT OUTER JOIN products as p
ON h.product_id = p.id
WHERE h.id in (?)
ORDER BY h.id DESC
SQL
    products = db.xquery(products_query, his_ids)

    total_pay = 0
    products.each do |product|
      total_pay += product[:price]
    end

    user = db.xquery('SELECT * FROM users WHERE id = ?', params[:user_id]).first
    erb :mypage, locals: { products: products, user: user, total_pay: total_pay }
  end

  get '/products/:product_id' do
    product = db.xquery('SELECT * FROM products WHERE id = ?', params[:product_id]).first
    comments = db.xquery('SELECT * FROM comments WHERE product_id = ?', params[:product_id])
    erb :product, locals: { product: product, comments: comments }
  end

  post '/products/buy/:product_id' do
    authenticated!
    buy_product(params[:product_id], session[:user_id])
    redirect "/users/#{session[:user_id]}"
  end

  post '/comments/:product_id' do
    authenticated!
    create_comment(params[:product_id], session[:user_id], params[:content])
    redirect "/users/#{session[:user_id]}"
  end

  get '/initialize' do
    db.query('DELETE FROM users WHERE id > 5000')
    db.query('DELETE FROM products WHERE id > 10000')
    db.query('DELETE FROM comments WHERE id > 200000')
    db.query('DELETE FROM histories WHERE id > 500000')
    redis.flushall
    users = db.query('select * from users')
    users.each do |user|
      redis.set(user[:id].to_s+'nm', user[:name])
    end
    comments = db.query('select * from comments order by created_at limit 100000')
    comments.each do |c|
      create_comment(c[:product_id], c[:user_id], c[:content])
    end
    products = db.query('select id from products')
    products.each do |pro|
      redis.set(pro[:id].to_s+'cnt', 20)
    end
    "Finish"
  end
end
