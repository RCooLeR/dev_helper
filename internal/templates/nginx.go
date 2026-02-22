package templates

import (
	"bytes"
	"text/template"
)

type NginxParams struct {
	Domain        string
	Root          string
	AccessLog     string
	ErrorLog      string
	PHPUpstream   string
	ExternalCert  string
	ExternalKey   string
	ExternalDh    string
	AppEnv        string
	ClientMaxBody string
}

var tmpl = template.Must(template.New("vhost").Parse(`
server {
    listen 443 ssl;
    server_name {{.Domain}};

    ssl_certificate      {{.ExternalCert}};
    ssl_certificate_key  {{.ExternalKey}};
    ssl_dhparam {{.ExternalDh}};

    root {{.Root}};
    index index.html index.php;

    access_log  {{.AccessLog}};
    error_log   {{.ErrorLog}} error;

    server_tokens off;
    charset utf-8;
    sendfile off;
    aio threads;
    aio_write on;
    directio 8M;
    directio_alignment 4k;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 4096;
    proxy_read_timeout 600;
    proxy_connect_timeout 600;
    client_max_body_size {{.ClientMaxBody}};

    location ~ \.php$ {
		proxy_read_timeout 1000;
    	fastcgi_read_timeout 1000;
        fastcgi_split_path_info ^(.+\.php)(/.+)$;
        fastcgi_pass {{.PHPUpstream}};
        fastcgi_index index.php;
        try_files $uri $uri/ /index.php$is_args$args;
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME $request_filename;
        fastcgi_param APP_ENV {{.AppEnv}};
        fastcgi_buffers 32 32k;
        fastcgi_buffer_size 32k;
    }

	location = /favicon.ico {
		log_not_found off;
		access_log off;
		expires max;
	}
	location = /robots.txt {
		allow all;
		log_not_found off;
		access_log off;
	}
    location ~ /\. {
		access_log off;
		log_not_found off;
		deny all;
	}
	location ~* ^.+\.(ogg|ogv|svg|svgz|eot|otf|woff|woff2|mp4|ttf|rss|atom|jpg|jpeg|gif|png|ico|zip|tgz|gz|rar|bz2|doc|xls|exe|ppt|tar|mid|midi|wav|bmp|rtf)$ {
		access_log off;
		log_not_found off;
		expires max;
	}

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }
}

server {
    listen 80;
    server_name {{.Domain}};

    root {{.Root}};
    index index.html index.php;

    access_log  {{.AccessLog}};
    error_log   {{.ErrorLog}} error;

    server_tokens off;
    charset utf-8;
    sendfile off;
    aio threads;
    aio_write on;
    directio 8M;
    directio_alignment 4k;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 4096;
    proxy_read_timeout 600;
    proxy_connect_timeout 600;
    client_max_body_size {{.ClientMaxBody}};

    location ~ \.php$ {
        fastcgi_split_path_info ^(.+\.php)(/.+)$;
        fastcgi_pass {{.PHPUpstream}};
        fastcgi_index index.php;
        try_files $uri $uri/ /index.php$is_args$args;
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME $request_filename;
        fastcgi_param APP_ENV {{.AppEnv}};
    }

	location = /favicon.ico {
		log_not_found off;
		access_log off;
		expires max;
	}
	location = /robots.txt {
		allow all;
		log_not_found off;
		access_log off;
	}
    location ~ /\. {
		access_log off;
		log_not_found off;
		deny all;
	}
	location ~* ^.+\.(ogg|ogv|svg|svgz|eot|otf|woff|woff2|mp4|ttf|rss|atom|jpg|jpeg|gif|png|ico|zip|tgz|gz|rar|bz2|doc|xls|exe|ppt|tar|mid|midi|wav|bmp|rtf)$ {
		access_log off;
		log_not_found off;
		expires max;
	}

    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }
}
`))

func RenderNginx(p NginxParams) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", err
	}
	return buf.String(), nil
}
