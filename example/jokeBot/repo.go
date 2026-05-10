package main

import "math/rand/v2"

type fakeJokeRepo struct {
	jokes []string
}

func (r *fakeJokeRepo) GetARandomJoke() string {
	if len(r.jokes) > 0 {
		return r.jokes[rand.IntN(len(r.jokes))]
	}

	return "Why did the tomato turn red? Because it saw the salad dressing!"
}

func (r *fakeJokeRepo) GetARandomGoodJoke() string {
	if len(r.jokes) > 0 {
		return r.jokes[rand.IntN(len(r.jokes))]
	}

	return "What do you call a fish with no eyes? A fsh."
}

func (r *fakeJokeRepo) SaveJoke(joke string) {
	r.jokes = append(r.jokes, joke)
}
