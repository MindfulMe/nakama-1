DROP DATABASE IF EXISTS nakama CASCADE;
CREATE DATABASE IF NOT EXISTS nakama;
SET DATABASE = nakama;

CREATE TABLE IF NOT EXISTS users (
    id SERIAL NOT NULL PRIMARY KEY,
    email STRING(128) NOT NULL UNIQUE,
    username STRING(15) NOT NULL UNIQUE,
    avatar_url STRING,
    followers_count INT NOT NULL CHECK (followers_count >= 0) DEFAULT 0,
    following_count INT NOT NULL CHECK (following_count >= 0) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    notifications_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS follows (
    follower_id INT NOT NULL REFERENCES users,
    following_id INT NOT NULL REFERENCES users,
    PRIMARY KEY(follower_id, following_id)
);

CREATE TABLE IF NOT EXISTS posts (
    id SERIAL NOT NULL PRIMARY KEY,
    content STRING(480) NOT NULL,
    spoiler_of STRING(128),
    likes_count INT NOT NULL CHECK (likes_count >= 0) DEFAULT 0,
    comments_count INT NOT NULL CHECK (comments_count >= 0) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id INT NOT NULL REFERENCES users,
    INDEX (created_at DESC)
);

CREATE TABLE IF NOT EXISTS post_likes (
    user_id INT NOT NULL REFERENCES users,
    post_id INT NOT NULL REFERENCES posts,
    PRIMARY KEY (user_id, post_id)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    user_id INT NOT NULL REFERENCES users,
    post_id INT NOT NULL REFERENCES posts,
    PRIMARY KEY (user_id, post_id)
);

CREATE TABLE IF NOT EXISTS feed (
    id SERIAL NOT NULL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users,
    post_id INT NOT NULL REFERENCES posts
);

CREATE TABLE IF NOT EXISTS comments (
    id SERIAL NOT NULL PRIMARY KEY,
    content STRING(256) NOT NULL,
    likes_count INT NOT NULL CHECK (likes_count >= 0) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id INT NOT NULL REFERENCES users,
    post_id INT NOT NULL REFERENCES posts,
    INDEX (created_at DESC)
);

CREATE TABLE IF NOT EXISTS comment_likes (
    user_id INT NOT NULL REFERENCES users,
    comment_id INT NOT NULL REFERENCES comments,
    PRIMARY KEY (user_id, comment_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL NOT NULL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users,
    actor_id INT NOT NULL REFERENCES users,
    verb STRING(15) NOT NULL,
    object_id INT,
    target_id INT,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    INDEX (issued_at DESC)
);

INSERT INTO users (id, email, username) VALUES
    (1, 'john@example.dev', 'john_doe'),
    (2, 'jane@example.dev', 'jane_doe');
INSERT INTO follows (follower_id, following_id) VALUES
    (2, 1);
UPDATE users SET following_count = following_count + 1 WHERE id = 2;
UPDATE users SET followers_count = followers_count + 1 WHERE id = 1;
INSERT INTO notifications (id, user_id, actor_id, verb) VALUES
    (1, 1, 2, 'follow');

INSERT INTO posts (id, content, user_id) VALUES
    (1, '1st post', 1);
INSERT INTO subscriptions (user_id, post_id) VALUES
    (1, 1);
INSERT INTO feed (id, user_id, post_id) VALUES
    (1, 1, 1),
    (2, 2, 1);

INSERT INTO comments (id, content, user_id, post_id) VALUES
    (1, '1st comment', 1, 1);
UPDATE posts SET comments_count = comments_count + 1 WHERE id = 1;
